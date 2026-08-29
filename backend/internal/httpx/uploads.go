package httpx

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"  // registers GIF decoding for dimension validation
	_ "image/jpeg" // registers JPEG decoding
	_ "image/png"  // registers PNG decoding
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// uploadPrefix is the public URL space that maps to cfg.UploadDir.
const uploadPrefix = "/uploads/"

// maxImageEdge rejects absurd images that would only serve to waste bandwidth.
const maxImageEdge = 4096

// allowedImageTypes maps a sniffed MIME type to the extension we store it under.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// handleUploadAvatar accepts a multipart form with a single "avatar" file,
// validates it as a real raster image and swaps it in atomically.
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	// Cap the whole request body, not just the file part.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+4<<10)

	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
				"Image is larger than the allowed size.")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_upload", "Expected a multipart form upload.")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_upload", "No file was sent in the 'avatar' field.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, s.cfg.MaxUploadBytes+1))
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	if int64(len(data)) > s.cfg.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
			"Image is larger than the allowed size.")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_upload", "The uploaded file is empty.")
		return
	}

	mime := http.DetectContentType(data)
	ext, ok := allowedImageTypes[mime]
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_image",
			"Only JPEG, PNG, GIF and WebP images are accepted.")
		return
	}
	// WebP has no stdlib decoder; the other formats are decoded far enough to
	// confirm they are genuine images with sane dimensions.
	if mime != "image/webp" {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_image",
				"That file could not be read as an image.")
			return
		}
		if cfg.Width > maxImageEdge || cfg.Height > maxImageEdge {
			writeError(w, http.StatusUnprocessableEntity, "image_too_large",
				"Image must be at most 4096x4096 pixels.")
			return
		}
	}

	name := "avatar-" + randomToken(12) + ext
	dest := filepath.Join(s.cfg.UploadDir, name)
	if err := writeFileAtomic(dest, data); err != nil {
		s.log.ErrorContext(r.Context(), "store avatar failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not store the image.")
		return
	}

	previous, err := s.st.SetAvatarURL(r.Context(), uploadPrefix+name)
	if err != nil {
		os.Remove(dest) // keep the directory consistent with the database
		s.writeStoreError(w, r, err)
		return
	}
	s.removeManagedUpload(previous)

	profile, err := s.st.Profile(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// handleDeleteAvatar clears the avatar and falls back to the generated initial.
func (s *Server) handleDeleteAvatar(w http.ResponseWriter, r *http.Request) {
	previous, err := s.st.SetAvatarURL(r.Context(), "")
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.removeManagedUpload(previous)

	profile, err := s.st.Profile(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// removeManagedUpload deletes a file this server previously wrote. Anything
// that is not a plain name inside the upload directory is ignored.
func (s *Server) removeManagedUpload(publicURL string) {
	if !strings.HasPrefix(publicURL, uploadPrefix) {
		return
	}
	name := path.Base(strings.TrimPrefix(publicURL, uploadPrefix))
	if name == "." || name == "/" || name == "" || strings.Contains(name, "..") {
		return
	}
	if err := os.Remove(filepath.Join(s.cfg.UploadDir, name)); err != nil && !os.IsNotExist(err) {
		s.log.Warn("could not remove replaced avatar", "file", name, "error", err)
	}
}

// writeFileAtomic writes via a temp file + rename so a partially written image
// is never visible to the file server.
func writeFileAtomic(dest string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o640); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

// uploadHandler serves the upload directory read-only, without directory
// listings and with an immutable cache policy (file names are random).
func (s *Server) uploadHandler() http.Handler {
	fileServer := http.FileServer(http.Dir(s.cfg.UploadDir))

	return http.StripPrefix(uploadPrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/") || clean == "/" {
			writeError(w, http.StatusNotFound, "not_found", "File not found.")
			return
		}
		if _, ok := allowedImageTypes[mimeByExt(path.Ext(clean))]; !ok {
			writeError(w, http.StatusNotFound, "not_found", "File not found.")
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", "inline")
		fileServer.ServeHTTP(w, r)
	}))
}

// mimeByExt is the inverse of allowedImageTypes, limited to what we store.
func mimeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}
