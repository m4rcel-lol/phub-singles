package httpx

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves the compiled Angular bundle from an embedded filesystem.
//
// Hashed build artefacts are cached forever; index.html must always be
// revalidated so a deploy is picked up on the next navigation. Unknown paths
// without a file extension fall back to index.html so client-side routes such
// as /admin work on a hard refresh.
func (s *Server) spaHandler(dist fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed",
				"Only GET and HEAD are supported here.")
			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" || name == "." {
			s.serveIndex(w, r, dist)
			return
		}

		info, err := fs.Stat(dist, name)
		if err != nil || info.IsDir() {
			if path.Ext(name) != "" {
				// A missing asset is a real 404, not an app route.
				writeError(w, http.StatusNotFound, "not_found", "File not found.")
				return
			}
			s.serveIndex(w, r, dist)
			return
		}

		w.Header().Set("Cache-Control", cachePolicy(name))
		http.ServeFileFS(w, r, dist, name)
	})
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request, dist fs.FS) {
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, dist, "index.html")
}

// cachePolicy picks a Cache-Control value from the asset kind. The Angular
// production build content-hashes every JS/CSS/font file it emits.
func cachePolicy(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".js", ".css", ".woff", ".woff2", ".ttf":
		return "public, max-age=31536000, immutable"
	case ".html", ".webmanifest", ".json":
		return "no-cache"
	default:
		return "public, max-age=86400"
	}
}
