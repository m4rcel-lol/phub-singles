package httpx

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"pornhub.singles/server/internal/ratelimit"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyUser
)

// chain applies middlewares so the first argument is the outermost layer.
func chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// statusRecorder captures the response status and size for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
		r.ResponseWriter.WriteHeader(status)
	}
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush keeps streaming handlers (none today, but cheap insurance) working.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = randomToken(8)
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, id)))
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		case strings.HasPrefix(r.URL.Path, "/api/health"):
			level = slog.LevelDebug // keep probe noise out of production logs
		}

		s.log.Log(r.Context(), level, "http request",
			"request_id", requestIDFrom(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", s.clientIP(r),
		)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec) // let the server handle its own abort sentinel
				}
				s.log.ErrorContext(r.Context(), "panic recovered",
					"request_id", requestIDFrom(r.Context()),
					"panic", rec,
					"path", r.URL.Path,
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets defence-in-depth headers at the application layer so
// they hold even when the app is reached without going through Caddy.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " + // Angular injects component styles
		"img-src 'self' data: https:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		"form-action 'self'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'; " +
		"object-src 'none'"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

// cors answers preflight requests for the origins listed in PHS_CORS_ORIGINS.
// With no origins configured the API is same-origin only, which is the default
// production posture (Caddy serves API and app from one host).
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && slices.Contains(s.cfg.CORSOrigins, origin) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimit guards a handler with a per-client-IP token bucket.
func (s *Server) rateLimit(l *ratelimit.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(s.clientIP(r)) {
			w.Header().Set("Retry-After", "5")
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"Too many requests. Please slow down.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireSameOrigin blocks cross-site state changes. Combined with the
// SameSite=Strict session cookie this removes the need for CSRF tokens.
func (s *Server) requireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// Some clients omit Origin on same-origin requests; fall back to
			// Referer, and accept the request when neither header is present
			// (curl, server-to-server) because those carry no ambient cookies
			// from a browser context.
			if ref := r.Header.Get("Referer"); ref != "" {
				if u, err := url.Parse(ref); err == nil && u.Scheme != "" {
					origin = u.Scheme + "://" + u.Host
				}
			}
		}
		if origin == "" || s.originAllowed(r, origin) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusForbidden, "cross_origin_blocked",
			"Cross-origin state changes are not allowed.")
	})
}

func (s *Server) originAllowed(r *http.Request, origin string) bool {
	if slices.Contains(s.cfg.CORSOrigins, origin) {
		return true
	}
	if s.cfg.PublicURL != "" && strings.EqualFold(origin, s.cfg.PublicURL) {
		return true
	}
	// Same-host requests: compare the Origin host against the request host.
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// clientIP resolves the caller address, honouring the proxy header only when
// the deployment is explicitly marked as running behind a trusted proxy.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustedProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if idx := strings.IndexByte(fwd, ','); idx >= 0 {
				fwd = fwd[:idx]
			}
			if ip := strings.TrimSpace(fwd); ip != "" {
				return ip
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-Ip")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

// randomToken returns a URL-safe token built from n*8 bits of entropy.
func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand never fails on supported platforms; failing loudly is
		// preferable to handing out a predictable session token.
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
