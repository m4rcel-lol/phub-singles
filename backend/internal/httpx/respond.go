package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"pornhub.singles/server/internal/store"
)

// apiError is the single error shape every JSON endpoint returns.
type apiError struct {
	Error   string `json:"error"`            // stable machine-readable code
	Message string `json:"message"`          // human-readable detail
	Fields  any    `json:"fields,omitempty"` // optional per-field messages
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status line is already out; all that is left is a log line.
		slog.Default().Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Error: code, Message: message})
}

func writeFieldErrors(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, apiError{
		Error:   "validation_failed",
		Message: "One or more fields are invalid.",
		Fields:  fields,
	})
}

// writeStoreError maps store-level sentinels onto HTTP status codes and hides
// unexpected database failures behind a generic 500.
func (s *Server) writeStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "The requested resource does not exist.")
	case errors.Is(err, store.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		s.log.ErrorContext(r.Context(), "unhandled error",
			"error", err, "path", r.URL.Path, "method", r.Method)
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
	}
}

// decodeJSON reads a small JSON body into dst, rejecting unknown fields so a
// typo in the admin client surfaces immediately instead of being ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if ct := r.Header.Get("Content-Type"); ct != "" && !hasJSONContentType(ct) {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Content-Type must be application/json.")
		return false
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
				"Request body is too large.")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.")
		return false
	}
	return true
}

func hasJSONContentType(ct string) bool {
	for i := 0; i < len(ct); i++ {
		if ct[i] == ';' {
			ct = ct[:i]
			break
		}
	}
	switch trimSpace(ct) {
	case "application/json", "text/json", "application/json; charset=utf-8":
		return true
	}
	return false
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
