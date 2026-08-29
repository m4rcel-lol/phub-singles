package httpx

import (
	"net/http"
	"strconv"

	"pornhub.singles/server/internal/store"
)

// --- profile ---------------------------------------------------------------

type profileRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Tagline     string `json:"tagline"`
	Bio         string `json:"bio"`
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.st.Profile(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	handle, handleProblem := validateHandle(req.Username)

	p := store.Profile{
		Username:    handle,
		DisplayName: sanitiseText(req.DisplayName),
		Tagline:     sanitiseText(req.Tagline),
		Bio:         sanitiseText(req.Bio),
	}

	fields := map[string]string{}
	if handleProblem != "" {
		fields["username"] = handleProblem
	}
	if p.DisplayName == "" {
		fields["displayName"] = "A display name is required."
	}
	checkLen(fields, "displayName", p.DisplayName, maxDisplayName)
	checkLen(fields, "tagline", p.Tagline, maxTagline)
	checkLen(fields, "bio", p.Bio, maxBio)
	if len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	updated, err := s.st.UpdateProfile(r.Context(), p)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// --- links -----------------------------------------------------------------

type linkRequest struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Icon    string `json:"icon"`
	Enabled *bool  `json:"enabled"` // pointer so omission defaults to enabled
}

// parse validates the payload and returns the value to persist.
func (req linkRequest) parse() (store.LinkInput, map[string]string) {
	in := store.LinkInput{
		Title:   sanitiseText(req.Title),
		Icon:    sanitiseText(req.Icon),
		Enabled: req.Enabled == nil || *req.Enabled,
	}

	fields := map[string]string{}
	if in.Title == "" {
		fields["title"] = "A title is required."
	}
	checkLen(fields, "title", in.Title, maxLinkTitle)
	checkLen(fields, "icon", in.Icon, maxIconRunes)

	normalised, problem := validateLinkURL(req.URL)
	if problem != "" {
		fields["url"] = problem
	}
	in.URL = normalised

	return in, fields
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.st.Links(r.Context(), false)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var req linkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	in, fields := req.parse()
	if len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	link, err := s.st.CreateLink(r.Context(), in)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (s *Server) handleUpdateLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req linkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	in, fields := req.parse()
	if len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	link, err := s.st.UpdateLink(r.Context(), id, in)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.st.DeleteLink(r.Context(), id); err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reorderRequest struct {
	IDs []int64 `json:"ids"`
}

// handleReorderLinks accepts the complete ordering produced by the admin
// drag-and-drop list.
func (s *Server) handleReorderLinks(w http.ResponseWriter, r *http.Request) {
	var req reorderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 {
		writeFieldErrors(w, map[string]string{"ids": "Send the full list of link ids."})
		return
	}

	links, err := s.st.ReorderLinks(r.Context(), req.IDs)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

// --- stats -----------------------------------------------------------------

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	days := 14
	if raw := r.URL.Query().Get("days"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		}
	}

	stats, err := s.st.Stats(r.Context(), days)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// pathID extracts and validates the {id} path segment.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", "Resource id must be a positive integer.")
		return 0, false
	}
	return id, true
}
