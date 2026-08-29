package httpx

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipPool reuses writers; the same handful of small responses is compressed
// over and over, so allocating a writer per request is pure waste.
var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// compressibleTypes lists the content types worth compressing. Images, fonts
// and anything already compressed are skipped.
func compressible(contentType string) bool {
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = contentType[:idx]
	}
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case "text/html", "text/css", "text/plain", "text/xml",
		"application/javascript", "text/javascript",
		"application/json", "application/manifest+json",
		"image/svg+xml", "application/xml":
		return true
	default:
		return false
	}
}

// minCompressSize skips payloads too small for compression to pay off.
const minCompressSize = 1024

// gzipResponseWriter defers the decision to compress until the first write,
// when the Content-Type of the response is finally known.
type gzipResponseWriter struct {
	http.ResponseWriter

	gz          *gzip.Writer
	wroteHeader bool
	passthrough bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	header := w.Header()
	// 204/304 have no body, and a pre-encoded body must not be touched.
	if status == http.StatusNoContent || status == http.StatusNotModified ||
		status == http.StatusPartialContent || header.Get("Content-Encoding") != "" ||
		!compressible(header.Get("Content-Type")) {
		w.passthrough = true
		w.ResponseWriter.WriteHeader(status)
		return
	}

	// Content-Length no longer describes the encoded body.
	header.Del("Content-Length")
	header.Set("Content-Encoding", "gzip")
	header.Add("Vary", "Accept-Encoding")

	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(w.ResponseWriter)
	w.gz = gz
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		// Mirror net/http: an implicit 200 with a sniffed content type.
		if w.Header().Get("Content-Type") == "" && len(b) > 0 {
			w.Header().Set("Content-Type", http.DetectContentType(b))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.passthrough {
		return w.ResponseWriter.Write(b)
	}
	return w.gz.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if w.gz != nil {
		w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// close finishes the gzip stream and returns the writer to the pool.
func (w *gzipResponseWriter) close() {
	if w.gz == nil {
		return
	}
	w.gz.Close()
	gzipPool.Put(w.gz)
	w.gz = nil
}

// compress gzips eligible responses. Caddy is configured to proxy only, so
// compression is the application's job — which also means it works when the
// binary is run without a proxy in front of it.
func (s *Server) compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Range requests are served verbatim: compressing a byte range would
		// make the offsets meaningless.
		if r.Header.Get("Range") != "" || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}
