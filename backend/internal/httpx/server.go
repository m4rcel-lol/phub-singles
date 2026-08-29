// Package httpx wires the HTTP surface: routing, middleware, handlers and the
// static frontend.
package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"pornhub.singles/server/internal/config"
	"pornhub.singles/server/internal/ratelimit"
	"pornhub.singles/server/internal/store"
	"pornhub.singles/server/internal/web"
)

// Server holds the dependencies shared by every handler.
type Server struct {
	cfg     config.Config
	st      *store.Store
	log     *slog.Logger
	handler http.Handler
	started time.Time

	// Secret behind the analytics fingerprints; generated on first start and
	// kept in the database, never in the environment or the image.
	eventSecret []byte

	// Public write endpoints are cheap but unauthenticated, so each gets its
	// own bucket; login is limited far more aggressively than analytics.
	viewLimiter  *ratelimit.Limiter
	clickLimiter *ratelimit.Limiter
	loginLimiter *ratelimit.Limiter
}

// New builds the fully wired HTTP handler.
func New(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger) (*Server, error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o750); err != nil {
		return nil, err
	}
	secret, err := st.EnsureSecret(ctx, "event_fingerprint_secret")
	if err != nil {
		return nil, err
	}

	s := &Server{
		eventSecret:  []byte(secret),
		cfg:          cfg,
		st:           st,
		log:          log,
		started:      time.Now(),
		viewLimiter:  ratelimit.New(0.5, 5, 15*time.Minute),  // ~1 view / 2s, burst 5
		clickLimiter: ratelimit.New(2, 20, 15*time.Minute),   // clicking several links is normal
		loginLimiter: ratelimit.New(0.05, 8, 30*time.Minute), // 8 tries, then 1 per 20s
	}
	s.handler = chain(s.routes(),
		s.recoverer,
		s.requestID,
		s.accessLog,
		s.securityHeaders,
		s.cors,
		s.compress,
	)
	return s, nil
}

// Handler returns the root http.Handler for the server.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// --- health -------------------------------------------------------------
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/ready", s.handleReady)

	// --- public API ---------------------------------------------------------
	mux.HandleFunc("GET /api/page", s.handlePage)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.Handle("POST /api/views", s.rateLimit(s.viewLimiter, http.HandlerFunc(s.handleRegisterView)))
	mux.Handle("POST /api/links/{id}/click", s.rateLimit(s.clickLimiter, http.HandlerFunc(s.handleRegisterClick)))

	// --- admin authentication ----------------------------------------------
	mux.Handle("POST /api/admin/login",
		s.requireSameOrigin(s.rateLimit(s.loginLimiter, http.HandlerFunc(s.handleLogin))))
	mux.Handle("POST /api/admin/logout", s.requireSameOrigin(http.HandlerFunc(s.handleLogout)))
	mux.Handle("POST /api/admin/password",
		s.requireSameOrigin(s.requireAuth(http.HandlerFunc(s.handleChangePassword))))

	// --- admin resources ----------------------------------------------------
	admin := func(h http.HandlerFunc) http.Handler {
		return s.requireSameOrigin(s.requireAuth(s.requireAdmin(h)))
	}
	mux.Handle("GET /api/admin/profile", admin(s.handleGetProfile))
	mux.Handle("PUT /api/admin/profile", admin(s.handleUpdateProfile))
	mux.Handle("POST /api/admin/profile/avatar", admin(s.handleUploadAvatar))
	mux.Handle("DELETE /api/admin/profile/avatar", admin(s.handleDeleteAvatar))

	mux.Handle("GET /api/admin/links", admin(s.handleListLinks))
	mux.Handle("POST /api/admin/links", admin(s.handleCreateLink))
	mux.Handle("PUT /api/admin/links/order", admin(s.handleReorderLinks))
	mux.Handle("PUT /api/admin/links/{id}", admin(s.handleUpdateLink))
	mux.Handle("DELETE /api/admin/links/{id}", admin(s.handleDeleteLink))

	mux.Handle("GET /api/admin/stats", admin(s.handleStats))

	// Account management: administrators may act on members, the owner on
	// everyone below them (see mayManage).
	mux.Handle("GET /api/admin/users", admin(s.handleListUsers))
	mux.Handle("POST /api/admin/users", admin(s.handleCreateUser))
	mux.Handle("DELETE /api/admin/users/{username}", admin(s.handleDeleteUser))
	mux.Handle("PUT /api/admin/users/{username}/verified", admin(s.handleSetVerified))
	mux.Handle("POST /api/admin/users/{username}/password", admin(s.handleResetPassword))

	// Owner panel: roles and site settings.
	owner := func(h http.HandlerFunc) http.Handler {
		return s.requireSameOrigin(s.requireAuth(s.requireOwner(h)))
	}
	mux.Handle("PUT /api/admin/users/{username}/role", owner(s.handleSetRole))
	mux.Handle("GET /api/admin/settings", owner(s.handleGetSettings))
	mux.Handle("PUT /api/admin/settings", owner(s.handleUpdateSettings))

	// Anything else under /api is a 404 in JSON, never the SPA shell.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Unknown API endpoint.")
	})

	// robots.txt is generated from the site settings, so the owner can pull the
	// site out of search results without a redeploy. It shadows the bundled file.
	mux.HandleFunc("GET /robots.txt", s.handleRobots)

	// --- uploaded media and the compiled Angular app ------------------------
	mux.Handle("GET /uploads/", s.uploadHandler())
	mux.Handle("HEAD /uploads/", s.uploadHandler())

	spa, err := web.Dist()
	if err != nil {
		// Only possible if the embedded FS is malformed, i.e. a build error.
		panic("embedded frontend unavailable: " + err.Error())
	}
	mux.Handle("/", s.spaHandler(spa))

	return mux
}

// Maintain runs periodic housekeeping (expired sessions, idle rate-limit
// buckets) until the context is cancelled.
func (s *Server) Maintain(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := s.st.PurgeExpiredSessions(ctx); err != nil {
				s.log.WarnContext(ctx, "purge sessions failed", "error", err)
			} else if n > 0 {
				s.log.DebugContext(ctx, "purged expired sessions", "count", n)
			}
			if n, err := s.st.PurgeExpiredEvents(ctx); err != nil {
				s.log.WarnContext(ctx, "purge view events failed", "error", err)
			} else if n > 0 {
				s.log.DebugContext(ctx, "purged expired view fingerprints", "count", n)
			}
			s.viewLimiter.Cleanup()
			s.clickLimiter.Cleanup()
			s.loginLimiter.Cleanup()
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"uptime_sec": int64(time.Since(s.started).Seconds()),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.st.Ping(ctx); err != nil {
		s.log.ErrorContext(ctx, "readiness probe failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "not_ready", "Database is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}
