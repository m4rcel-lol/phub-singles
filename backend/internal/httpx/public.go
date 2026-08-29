package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"pornhub.singles/server/internal/store"
)

// publicLink is the trimmed link shape the public page receives: no click
// counts, no timestamps, no disabled entries.
type publicLink struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
}

// sitePayload is the public slice of the owner-controlled site settings.
type sitePayload struct {
	Headline string `json:"headline"`
	Lede     string `json:"lede"`
}

type pagePayload struct {
	Site    sitePayload   `json:"site"`
	Profile store.Profile `json:"profile"`
	Badges  []store.Badge `json:"badges"`
	Links   []publicLink  `json:"links"`
}

// handlePage returns everything the public page needs in one round trip.
//
// The optional ?handle= parameter lets /<handle> be resolved server-side: an
// unknown handle answers 404 instead of silently rendering the profile.
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	settings, err := s.st.SiteSettings(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	// Maintenance hides the page from visitors but not from the people who
	// have to check their work while it is on.
	if settings.Maintenance && !s.callerIsAdmin(r) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "300")
		writeError(w, http.StatusServiceUnavailable, "maintenance", settings.MaintenanceMessage)
		return
	}

	profile, err := s.st.Profile(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	if handle := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("handle"))); handle != "" &&
		handle != strings.ToLower(profile.Username) {
		writeError(w, http.StatusNotFound, "not_found", "No page lives at that address.")
		return
	}
	links, err := s.st.Links(r.Context(), true)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	badges, err := s.st.ProfileBadges(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}

	out := pagePayload{
		Site:    sitePayload{Headline: settings.Headline, Lede: settings.Lede},
		Profile: profile,
		Badges:  badges,
		Links:   make([]publicLink, 0, len(links)),
	}
	for _, l := range links {
		out.Links = append(out.Links, publicLink{ID: l.ID, Title: l.Title, URL: l.URL, Icon: l.Icon})
	}

	// The payload is tiny and must reflect admin edits immediately.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// handleRegisterView counts one page view.
//
// The response is always 204, whether or not the view was counted: a client
// that learns it was filtered out is a client that can tune around the filter.
func (s *Server) handleRegisterView(w http.ResponseWriter, r *http.Request) {
	if !s.countingAllowed(r, "view") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	fresh, err := s.st.ClaimEvent(r.Context(), s.fingerprint(r, "view"), s.cfg.ViewWindow)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	if !fresh {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := s.st.RegisterView(r.Context()); err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRegisterClick counts a click on an enabled link. The browser opens the
// destination itself, so this endpoint only needs to acknowledge the beacon.
func (s *Server) handleRegisterClick(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", "Link id must be a positive integer.")
		return
	}
	if !s.countingAllowed(r, "click") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Clicks use a much shorter window than views: opening the same link again
	// hours later is normal behaviour, twice in a minute is a double click.
	fresh, err := s.st.ClaimEvent(r.Context(),
		s.fingerprint(r, "click:"+strconv.FormatInt(id, 10)), clickWindow)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	if !fresh {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if _, err := s.st.RegisterClick(r.Context(), id); err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clickWindow is how long a click on one link by one visitor is de-duplicated.
const clickWindow = 15 * time.Minute

// countingAllowed applies the cheap, request-shaped filters that run before a
// counter is touched.
func (s *Server) countingAllowed(r *http.Request, kind string) bool {
	if !s.fromOwnPage(r) {
		s.log.DebugContext(r.Context(), "analytics beacon ignored",
			"reason", "off_site", "kind", kind, "ip", s.clientIP(r))
		return false
	}
	if looksAutomated(r.UserAgent()) {
		s.log.DebugContext(r.Context(), "analytics beacon ignored",
			"reason", "automated_client", "kind", kind, "user_agent", r.UserAgent())
		return false
	}
	return true
}
