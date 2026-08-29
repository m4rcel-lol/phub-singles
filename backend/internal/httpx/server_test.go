package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pornhub.singles/server/internal/config"
	"pornhub.singles/server/internal/store"
)

// newTestServer boots an isolated server against a temporary database.
func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()

	dir := t.TempDir()
	st, err := store.Open(context.Background(), dir+"/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	if _, _, err := st.EnsureAdmin(context.Background(), "admin", "test-password", false); err != nil {
		t.Fatalf("ensure admin: %v", err)
	}

	cfg := config.Config{
		UploadDir:      dir + "/uploads",
		MaxUploadBytes: 1 << 20,
		SessionTTL:     time.Hour,
		ViewWindow:     time.Hour,
		SecureCookie:   false,
		PublicURL:      "http://example.test",
	}
	srv, err := New(context.Background(), cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, st
}

// do issues a request against the test server, optionally carrying cookies.
func do(t *testing.T, client *http.Client, method, url string, body any) (*http.Response, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Look like a browser on the site's own page: the analytics endpoints
	// ignore beacons that do not.
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, payload
}

func TestPublicPageExposesOnlyEnabledLinks(t *testing.T) {
	ts, st := newTestServer(t)

	links, err := st.Links(context.Background(), false)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) == 0 {
		t.Fatal("expected seeded links")
	}
	if _, err := st.UpdateLink(context.Background(), links[0].ID, store.LinkInput{
		Title: links[0].Title, URL: links[0].URL, Icon: links[0].Icon, Enabled: false,
	}); err != nil {
		t.Fatalf("disable link: %v", err)
	}

	resp, body := do(t, ts.Client(), http.MethodGet, ts.URL+"/api/page", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	var page struct {
		Profile struct {
			DisplayName string `json:"displayName"`
		} `json:"profile"`
		Links []struct {
			ID     int64  `json:"id"`
			Title  string `json:"title"`
			Clicks *int64 `json:"clicks"`
		} `json:"links"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if page.Profile.DisplayName == "" {
		t.Error("profile display name is empty")
	}
	if len(page.Links) != len(links)-1 {
		t.Errorf("expected %d links, got %d", len(links)-1, len(page.Links))
	}
	for _, l := range page.Links {
		if l.ID == links[0].ID {
			t.Error("disabled link leaked to the public payload")
		}
		if l.Clicks != nil {
			t.Error("click counts leaked to the public payload")
		}
	}
}

func TestCountersIncrement(t *testing.T) {
	ts, st := newTestServer(t)

	if resp, body := do(t, ts.Client(), http.MethodPost, ts.URL+"/api/views", nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("view status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body := do(t, ts.Client(), http.MethodPost, ts.URL+"/api/links/1/click", nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("click status = %d, body = %s", resp.StatusCode, body)
	}

	stats, err := st.Stats(context.Background(), 7)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalViews != 1 || stats.TotalClicks != 1 {
		t.Fatalf("views = %d, clicks = %d; want 1 and 1", stats.TotalViews, stats.TotalClicks)
	}

	resp, _ := do(t, ts.Client(), http.MethodPost, ts.URL+"/api/links/9999/click", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown link click = %d, want 404", resp.StatusCode)
	}
}

func TestAdminEndpointsRequireAuth(t *testing.T) {
	ts, _ := newTestServer(t)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/admin/links"},
		{http.MethodGet, "/api/admin/stats"},
		{http.MethodGet, "/api/admin/profile"},
		{http.MethodPost, "/api/admin/links"},
	} {
		resp, _ := do(t, ts.Client(), tc.method, ts.URL+tc.path, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestLoginAndLinkLifecycle(t *testing.T) {
	ts, _ := newTestServer(t)
	client := ts.Client()
	client.Jar = newJar(t)

	resp, body := do(t, client, http.MethodPost, ts.URL+"/api/admin/login",
		map[string]string{"username": "admin", "password": "wrong"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = do(t, client, http.MethodPost, ts.URL+"/api/admin/login",
		map[string]string{"username": "admin", "password": "test-password"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, body = %s", resp.StatusCode, body)
	}

	// Create.
	resp, body = do(t, client, http.MethodPost, ts.URL+"/api/admin/links",
		map[string]any{"title": "New link", "url": "example.org/path", "icon": "🎬"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d, body = %s", resp.StatusCode, body)
	}
	var created store.Link
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode link: %v", err)
	}
	if created.URL != "https://example.org/path" {
		t.Errorf("url = %q, want scheme to be added", created.URL)
	}

	// Validation.
	resp, body = do(t, client, http.MethodPut, ts.URL+"/api/admin/links/"+itoa(int(created.ID)),
		map[string]any{"title": "", "url": "javascript:alert(1)"})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid update = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "validation_failed") {
		t.Errorf("unexpected error body: %s", body)
	}

	// Reorder requires the complete id set.
	resp, _ = do(t, client, http.MethodPut, ts.URL+"/api/admin/links/order",
		map[string]any{"ids": []int64{created.ID}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("partial reorder = %d, want 400", resp.StatusCode)
	}

	resp, body = do(t, client, http.MethodGet, ts.URL+"/api/admin/links", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list = %d, body = %s", resp.StatusCode, body)
	}
	var list struct {
		Links []store.Link `json:"links"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	ids := make([]int64, 0, len(list.Links))
	ids = append(ids, created.ID)
	for _, l := range list.Links {
		if l.ID != created.ID {
			ids = append(ids, l.ID)
		}
	}
	resp, body = do(t, client, http.MethodPut, ts.URL+"/api/admin/links/order", map[string]any{"ids": ids})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reorder = %d, body = %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode reordered: %v", err)
	}
	if list.Links[0].ID != created.ID || list.Links[0].Position != 0 {
		t.Errorf("reorder did not move link to the front: %+v", list.Links[0])
	}

	// Delete, then confirm it is gone.
	resp, _ = do(t, client, http.MethodDelete, ts.URL+"/api/admin/links/"+itoa(int(created.ID)), nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	resp, _ = do(t, client, http.MethodDelete, ts.URL+"/api/admin/links/"+itoa(int(created.ID)), nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", resp.StatusCode)
	}

	// Logout invalidates the session.
	resp, _ = do(t, client, http.MethodPost, ts.URL+"/api/admin/logout", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout = %d", resp.StatusCode)
	}
	resp, _ = do(t, client, http.MethodGet, ts.URL+"/api/admin/links", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("after logout = %d, want 401", resp.StatusCode)
	}
}

func TestSPAFallbackAndAssets(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, body := do(t, ts.Client(), http.MethodGet, ts.URL+"/admin/links", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("spa route = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "<html") {
		t.Errorf("spa route did not return the app shell: %s", body)
	}

	resp, _ = do(t, ts.Client(), http.MethodGet, ts.URL+"/missing-asset.js", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing asset = %d, want 404", resp.StatusCode)
	}

	resp, _ = do(t, ts.Client(), http.MethodGet, ts.URL+"/uploads/", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("upload listing = %d, want 404", resp.StatusCode)
	}
}

func TestSecurityHeadersArePresent(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, _ := do(t, ts.Client(), http.MethodGet, ts.URL+"/api/health", nil)
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Error("missing Content-Security-Policy")
	}
}

func TestCrossOriginStateChangeIsBlocked(t *testing.T) {
	ts, _ := newTestServer(t)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/admin/login",
		strings.NewReader(`{"username":"admin","password":"test-password"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin login = %d, want 403", resp.StatusCode)
	}
}

func TestPageResolvesByHandle(t *testing.T) {
	ts, st := newTestServer(t)

	profile, err := st.Profile(context.Background())
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.Username == "" {
		t.Fatal("seeded profile has no handle")
	}

	resp, _ := do(t, ts.Client(), http.MethodGet, ts.URL+"/api/page?handle="+profile.Username, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("known handle = %d, want 200", resp.StatusCode)
	}

	resp, _ = do(t, ts.Client(), http.MethodGet, ts.URL+"/api/page?handle=someone-else", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown handle = %d, want 404", resp.StatusCode)
	}
}

func TestProfileHandleValidation(t *testing.T) {
	ts, _ := newTestServer(t)
	client := ts.Client()
	client.Jar = newJar(t)

	if resp, body := do(t, client, http.MethodPost, ts.URL+"/api/admin/login",
		map[string]string{"username": "admin", "password": "test-password"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, body = %s", resp.StatusCode, body)
	}

	for _, handle := range []string{"admin", "a", "Has Spaces", "-leading", "way-too-long-handle-that-should-be-rejected"} {
		resp, _ := do(t, client, http.MethodPut, ts.URL+"/api/admin/profile",
			map[string]string{"username": handle, "displayName": "Someone", "tagline": "", "bio": ""})
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("handle %q = %d, want 422", handle, resp.StatusCode)
		}
	}

	resp, body := do(t, client, http.MethodPut, ts.URL+"/api/admin/profile",
		map[string]string{"username": "New_Handle-1", "displayName": "Someone", "tagline": "", "bio": ""})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid handle = %d, body = %s", resp.StatusCode, body)
	}
	var updated store.Profile
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if updated.Username != "new_handle-1" {
		t.Errorf("handle = %q, want it lowercased", updated.Username)
	}
}

func TestViewsAreDeduplicatedPerVisitor(t *testing.T) {
	ts, st := newTestServer(t)

	for i := 0; i < 3; i++ {
		if resp, _ := do(t, ts.Client(), http.MethodPost, ts.URL+"/api/views", nil); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("view %d = %d", i, resp.StatusCode)
		}
	}

	stats, err := st.Stats(context.Background(), 7)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalViews != 1 {
		t.Fatalf("views = %d, want 1 (repeat views must not count)", stats.TotalViews)
	}
}

func TestBeaconsWithoutPageContextAreIgnored(t *testing.T) {
	ts, st := newTestServer(t)

	// A bare POST — no Sec-Fetch-Site, no Origin, no Referer — is the shape of
	// a shell loop, not of a browser on the page.
	post := func(headers map[string]string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/views", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want 204 regardless of filtering", resp.StatusCode)
		}
	}

	post(map[string]string{"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) Chrome/140.0"})
	post(map[string]string{"Sec-Fetch-Site": "same-origin", "User-Agent": "Googlebot/2.1"})
	post(map[string]string{"Sec-Fetch-Site": "cross-site", "User-Agent": "Mozilla/5.0 Chrome/140.0"})

	stats, err := st.Stats(context.Background(), 7)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalViews != 0 {
		t.Fatalf("views = %d, want 0", stats.TotalViews)
	}
}

func TestNonAdminCannotSignIn(t *testing.T) {
	ts, st := newTestServer(t)

	if _, err := st.CreateUser(context.Background(), "member", "member-password", store.RoleMember); err != nil {
		t.Fatalf("create member: %v", err)
	}

	resp, body := do(t, ts.Client(), http.MethodPost, ts.URL+"/api/admin/login",
		map[string]string{"username": "member", "password": "member-password"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("member login = %d, want 403 (body %s)", resp.StatusCode, body)
	}
}

func TestRankDecidesWhoMayActOnWhom(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	for _, account := range []struct{ name, role string }{
		{"carol", store.RoleAdmin},
		{"dave", store.RoleAdmin},
		{"mel", store.RoleMember},
	} {
		if _, err := st.CreateUser(ctx, account.name, account.name+"-password", account.role); err != nil {
			t.Fatalf("create %s: %v", account.name, err)
		}
	}

	client := ts.Client()
	client.Jar = newJar(t)
	if resp, body := do(t, client, http.MethodPost, ts.URL+"/api/admin/login",
		map[string]string{"username": "carol", "password": "carol-password"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("admin login = %d, body = %s", resp.StatusCode, body)
	}

	// An administrator may not touch the owner, a peer administrator, or their
	// own account...
	for _, target := range []string{"admin", "dave", "carol"} {
		resp, _ := do(t, client, http.MethodPut,
			ts.URL+"/api/admin/users/"+target+"/verified", map[string]bool{"verified": true})
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("admin verifying %q = %d, want 403", target, resp.StatusCode)
		}
	}
	owner, err := st.User(ctx, "admin")
	if err != nil {
		t.Fatalf("load owner: %v", err)
	}
	if owner.VerifiedAt != "" || owner.Role != store.RoleOwner {
		t.Fatalf("owner was modified: %+v", owner)
	}

	// ...but may act on a member.
	if resp, body := do(t, client, http.MethodPut, ts.URL+"/api/admin/users/mel/verified",
		map[string]bool{"verified": true}); resp.StatusCode != http.StatusOK {
		t.Fatalf("admin verifying a member = %d, body = %s", resp.StatusCode, body)
	}

	// Roles are the owner's alone, even over a member.
	resp, _ := do(t, client, http.MethodPut, ts.URL+"/api/admin/users/mel/role",
		map[string]string{"role": store.RoleAdmin})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("admin granting a role = %d, want 403", resp.StatusCode)
	}
	// So are site settings.
	resp, _ = do(t, client, http.MethodGet, ts.URL+"/api/admin/settings", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("admin reading settings = %d, want 403", resp.StatusCode)
	}
	// And so is creating another administrator.
	resp, _ = do(t, client, http.MethodPost, ts.URL+"/api/admin/users",
		map[string]string{"username": "eve", "password": "eve-password", "role": store.RoleAdmin})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("admin creating an admin = %d, want 422", resp.StatusCode)
	}
}

func TestOwnerPanelGrantsRolesAndSettings(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	if _, err := st.CreateUser(ctx, "mel", "mel-password", store.RoleMember); err != nil {
		t.Fatalf("create member: %v", err)
	}

	client := ts.Client()
	client.Jar = newJar(t)
	if resp, body := do(t, client, http.MethodPost, ts.URL+"/api/admin/login",
		map[string]string{"username": "admin", "password": "test-password"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("owner login = %d, body = %s", resp.StatusCode, body)
	}

	// Promote, then demote.
	if resp, body := do(t, client, http.MethodPut, ts.URL+"/api/admin/users/mel/role",
		map[string]string{"role": store.RoleAdmin}); resp.StatusCode != http.StatusOK {
		t.Fatalf("promote = %d, body = %s", resp.StatusCode, body)
	}
	promoted, err := st.User(ctx, "mel")
	if err != nil {
		t.Fatalf("load member: %v", err)
	}
	if promoted.Role != store.RoleAdmin {
		t.Fatalf("role = %q, want admin", promoted.Role)
	}

	// The owner may not be demoted, whoever asks.
	resp, _ := do(t, client, http.MethodPut, ts.URL+"/api/admin/users/admin/role",
		map[string]string{"role": store.RoleMember})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("owner demoting itself = %d, want 403", resp.StatusCode)
	}

	// Settings round-trip, and the threshold really drives the badge.
	resp, body := do(t, client, http.MethodPut, ts.URL+"/api/admin/settings", map[string]any{
		"headline":           "Links, and nothing else.",
		"lede":               "A short intro.",
		"verifiedThreshold":  25,
		"maintenance":        false,
		"maintenanceMessage": "Back shortly.",
		"indexing":           false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update settings = %d, body = %s", resp.StatusCode, body)
	}

	if err := st.SetPageViewsForTest(ctx, 30); err != nil {
		t.Fatalf("set views: %v", err)
	}
	_, body = do(t, ts.Client(), http.MethodGet, ts.URL+"/api/page", nil)
	var page struct {
		Site   struct{ Headline string } `json:"site"`
		Badges []store.Badge             `json:"badges"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if page.Site.Headline != "Links, and nothing else." {
		t.Errorf("headline = %q, want the stored one", page.Site.Headline)
	}
	if len(page.Badges) != 2 || page.Badges[0].ID != store.BadgeVerified {
		t.Errorf("badges = %+v, want verified at the lowered threshold", page.Badges)
	}

	// Indexing off must show up in robots.txt.
	_, body = do(t, ts.Client(), http.MethodGet, ts.URL+"/robots.txt", nil)
	if !strings.Contains(string(body), "Disallow: /\n") {
		t.Errorf("robots.txt = %q, want a full disallow", body)
	}
}

func TestMaintenanceHidesThePageFromVisitors(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	settings, err := st.SiteSettings(ctx)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	settings.Maintenance = true
	settings.MaintenanceMessage = "Back in ten minutes."
	if _, err := st.UpdateSiteSettings(ctx, settings); err != nil {
		t.Fatalf("enable maintenance: %v", err)
	}

	resp, body := do(t, ts.Client(), http.MethodGet, ts.URL+"/api/page", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("visitor during maintenance = %d, want 503", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Back in ten minutes.") {
		t.Errorf("body = %s, want the maintenance message", body)
	}

	// An administrator can still see the site while it is off.
	client := ts.Client()
	client.Jar = newJar(t)
	if resp, body := do(t, client, http.MethodPost, ts.URL+"/api/admin/login",
		map[string]string{"username": "admin", "password": "test-password"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("login = %d, body = %s", resp.StatusCode, body)
	}
	if resp, _ := do(t, client, http.MethodGet, ts.URL+"/api/page", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("admin during maintenance = %d, want 200", resp.StatusCode)
	}
}

func TestBadgesReflectRoleAndViews(t *testing.T) {
	ts, st := newTestServer(t)
	ctx := context.Background()

	resp, body := do(t, ts.Client(), http.MethodGet, ts.URL+"/api/page", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("page = %d", resp.StatusCode)
	}
	var page struct {
		Badges []store.Badge `json:"badges"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if len(page.Badges) != 1 || page.Badges[0].ID != store.BadgeOwner {
		t.Fatalf("badges = %+v, want just the owner badge", page.Badges)
	}

	// Crossing the threshold unlocks Verified without anyone granting it.
	if err := st.SetPageViewsForTest(ctx, store.DefaultVerifiedThreshold); err != nil {
		t.Fatalf("set views: %v", err)
	}
	_, body = do(t, ts.Client(), http.MethodGet, ts.URL+"/api/page", nil)
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if len(page.Badges) != 2 || page.Badges[0].ID != store.BadgeVerified {
		t.Fatalf("badges = %+v, want verified + owner", page.Badges)
	}
}
