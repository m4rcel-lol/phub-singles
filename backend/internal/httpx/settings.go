package httpx

import (
	"net/http"

	"pornhub.singles/server/internal/store"
)

// Site settings are owner-only: they change what every visitor sees, so they
// sit behind requireOwner rather than requireAdmin.

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.st.SiteSettings(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": settings,
		"limits": map[string]any{
			"headline":           store.MaxHeadline,
			"lede":               store.MaxLede,
			"maintenanceMessage": store.MaxMaintenanceMessage,
			"minThreshold":       store.MinVerifiedThreshold,
			"maxThreshold":       store.MaxVerifiedThreshold,
		},
	})
}

type settingsRequest struct {
	Headline           string `json:"headline"`
	Lede               string `json:"lede"`
	VerifiedThreshold  int64  `json:"verifiedThreshold"`
	Maintenance        bool   `json:"maintenance"`
	MaintenanceMessage string `json:"maintenanceMessage"`
	Indexing           bool   `json:"indexing"`
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	updated, err := s.st.UpdateSiteSettings(r.Context(), store.SiteSettings{
		Headline:           sanitiseText(req.Headline),
		Lede:               sanitiseText(req.Lede),
		VerifiedThreshold:  req.VerifiedThreshold,
		Maintenance:        req.Maintenance,
		MaintenanceMessage: sanitiseText(req.MaintenanceMessage),
		Indexing:           req.Indexing,
	})
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.log.InfoContext(r.Context(), "site settings updated",
		"by", userFrom(r.Context()).Username,
		"maintenance", updated.Maintenance,
		"indexing", updated.Indexing,
		"verified_threshold", updated.VerifiedThreshold)

	writeJSON(w, http.StatusOK, map[string]any{"settings": updated})
}

// handleRobots serves robots.txt from the current settings, so the owner can
// take the site out of search results without a redeploy. It shadows the copy
// in the frontend bundle.
func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	settings, err := s.st.SiteSettings(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}

	body := "User-agent: *\nDisallow: /admin\nDisallow: /api/\nAllow: /\n"
	if !settings.Indexing {
		body = "User-agent: *\nDisallow: /\n"
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write([]byte(body))
}
