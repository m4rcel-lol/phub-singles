package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"pornhub.singles/server/internal/store"
)

// sessionCookie is scoped to /api: the browser never needs to send it while
// fetching static assets.
const sessionCookie = "phs_session"
const sessionCookiePath = "/api"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// sessionResponse is returned by both the public session probe and login, so
// the client has one shape to reason about.
type sessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
	Role          string `json:"role,omitempty"`
	IsAdmin       bool   `json:"isAdmin"`
	ExpiresIn     int64  `json:"expiresIn,omitempty"` // seconds
}

// requireAuth rejects requests without a valid session cookie and puts the
// authenticated user in the request context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			s.denyUnauthenticated(w)
			return
		}

		user, err := s.st.UserBySession(r.Context(), cookie.Value)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.log.ErrorContext(r.Context(), "session lookup failed", "error", err)
			}
			s.clearSessionCookie(w)
			s.denyUnauthenticated(w)
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) denyUnauthenticated(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in to continue.")
}

func userFrom(ctx context.Context) store.User {
	user, _ := ctx.Value(ctxKeyUser).(store.User)
	return user
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" ||
		len(req.Username) > maxUsername || len(req.Password) > maxPassword {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect username or password.")
		return
	}

	user, err := s.st.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.writeStoreError(w, r, err)
			return
		}
		s.log.WarnContext(r.Context(), "failed login", "username", req.Username, "ip", s.clientIP(r))
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect username or password.")
		return
	}

	if !user.IsAdmin() {
		// A demoted account keeps its data and its badges but has nothing to
		// manage, so say so plainly instead of signing it into an empty panel.
		s.log.WarnContext(r.Context(), "login refused for non-admin account",
			"username", user.Username, "role", user.Role)
		writeError(w, http.StatusForbidden, "not_an_admin",
			"This account no longer has administrative access.")
		return
	}

	// A session cookie marked Secure is dropped by the browser on plain HTTP,
	// which looks exactly like "login works but every page bounces back to the
	// form". Warn once, at the point where it can still be diagnosed.
	if s.cfg.SecureCookie && !isSecureRequest(r) {
		s.log.WarnContext(r.Context(),
			"issuing a Secure session cookie over an insecure request; the browser will discard it",
			"hint", "serve the site over HTTPS, or set PHS_SECURE_COOKIE=false for local HTTP",
			"host", r.Host)
	}

	token := randomToken(32)
	if err := s.st.CreateSession(r.Context(), token, user.ID, s.cfg.SessionTTL); err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.setSessionCookie(w, token)
	s.log.InfoContext(r.Context(), "admin signed in", "username", user.Username, "ip", s.clientIP(r))

	writeJSON(w, http.StatusOK, sessionResponse{
		Authenticated: true,
		Username:      user.Username,
		Role:          user.Role,
		IsAdmin:       user.IsAdmin(),
		ExpiresIn:     int64(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		if err := s.st.DeleteSession(r.Context(), cookie.Value); err != nil {
			s.log.WarnContext(r.Context(), "delete session failed", "error", err)
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handleSession reports who the caller is. It is public and always answers 200
// so the landing page can render its sign-in button without provoking a 401 on
// every anonymous visit.
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusOK, sessionResponse{})
		return
	}

	user, err := s.st.UserBySession(r.Context(), cookie.Value)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.log.ErrorContext(r.Context(), "session lookup failed", "error", err)
		}
		s.clearSessionCookie(w)
		writeJSON(w, http.StatusOK, sessionResponse{})
		return
	}

	writeJSON(w, http.StatusOK, sessionResponse{
		Authenticated: true,
		Username:      user.Username,
		Role:          user.Role,
		IsAdmin:       user.IsAdmin(),
		ExpiresIn:     int64(s.cfg.SessionTTL.Seconds()),
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	fields := map[string]string{}
	if len(req.NewPassword) < 8 {
		fields["newPassword"] = "Must be at least 8 characters."
	}
	if len(req.NewPassword) > maxPassword {
		fields["newPassword"] = "Must be at most 72 characters."
	}
	if req.CurrentPassword == "" {
		fields["currentPassword"] = "Enter your current password."
	}
	if len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	user := userFrom(r.Context())
	if err := s.st.ChangePassword(r.Context(), user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusForbidden, "invalid_credentials", "Current password is incorrect.")
			return
		}
		s.writeStoreError(w, r, err)
		return
	}

	// ChangePassword revoked every session, including this one.
	s.clearSessionCookie(w)
	s.log.InfoContext(r.Context(), "admin password changed", "username", user.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     sessionCookiePath,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(s.cfg.SessionTTL),
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     sessionCookiePath,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// isSecureRequest reports whether the request reached the server over HTTPS,
// including the case where TLS was terminated by the reverse proxy.
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.EqualFold(proto, "https")
	}
	// Browsers treat loopback as a secure context, so Secure cookies are kept.
	host := r.Host
	if idx := strings.LastIndexByte(host, ':'); idx >= 0 {
		host = host[:idx]
	}
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]" || host == "::1"
}

// callerIsAdmin reports whether the request carries a valid administrative
// session. It is used by public handlers that behave differently for staff,
// and never to grant access — that is requireAuth's job.
func (s *Server) callerIsAdmin(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	user, err := s.st.UserBySession(r.Context(), cookie.Value)
	return err == nil && user.IsAdmin()
}
