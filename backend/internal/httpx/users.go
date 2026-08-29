package httpx

import (
	"net/http"

	"pornhub.singles/server/internal/store"
)

// userView is an account as the admin panel sees it, with its badges resolved
// and the caller's permissions over it already worked out.
type userView struct {
	store.User
	Badges        []store.Badge `json:"badges"`
	OwnsPage      bool          `json:"ownsPage"`
	CanManage     bool          `json:"canManage"`     // caller may verify/reset/delete
	CanChangeRole bool          `json:"canChangeRole"` // caller may promote/demote
	AutoVerified  bool          `json:"autoVerified"`  // Verified came from the view count
}

// requireAdmin gates the management API behind the owner and admin roles.
// A demoted account keeps its session but loses access on the next request.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !userFrom(r.Context()).IsAdmin() {
			writeError(w, http.StatusForbidden, "forbidden",
				"This account does not have administrative privileges.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireOwner gates the owner panel: site settings and role changes.
func (s *Server) requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !userFrom(r.Context()).IsOwner() {
			writeError(w, http.StatusForbidden, "owner_only",
				"Only the owner can do that.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// mayManage decides who may act on whom.
//
// The rule is rank: owner > admin > member, and you may only act on an account
// strictly below you. That is what keeps administrators away from each other
// and away from the owner, no matter what they send. The owner may also act on
// their own account — the store still refuses to delete or demote an owner, so
// the dangerous cases stay closed.
func mayManage(actor, target store.User) bool {
	if store.Rank(actor.Role) > store.Rank(target.Role) {
		return true
	}
	return actor.IsOwner() && actor.ID == target.ID
}

// resolveTarget loads the account named in the path and checks the caller may
// act on it. It writes the error response itself when the answer is no.
func (s *Server) resolveTarget(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	target, err := s.st.User(r.Context(), r.PathValue("username"))
	if err != nil {
		s.writeStoreError(w, r, err)
		return store.User{}, false
	}
	if !mayManage(userFrom(r.Context()), target) {
		writeError(w, http.StatusForbidden, "rank_too_low",
			"You cannot act on an account at or above your own level.")
		return store.User{}, false
	}
	return target, true
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.Users(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	state, err := s.st.ProfileBadgeState(r.Context())
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}

	actor := userFrom(r.Context())
	out := make([]userView, 0, len(users))
	for _, u := range users {
		// The page has one owning account; only its views count towards the
		// automatic Verified badge.
		owns := state.OwnerID != 0 && u.ID == state.OwnerID
		out = append(out, userView{
			User:          u,
			Badges:        s.st.UserBadges(u, owns, state.Views, state.Threshold),
			OwnsPage:      owns,
			CanManage:     mayManage(actor, u),
			CanChangeRole: actor.IsOwner() && !u.IsOwner(),
			AutoVerified:  owns && state.AutoVerified,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users":     out,
		"threshold": state.Threshold,
		"views":     state.Views,
		"actor":     map[string]any{"username": actor.Username, "role": actor.Role},
	})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// handleCreateUser adds an account. An administrator may only create members;
// handing out administrative privileges is the owner's call.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	actor := userFrom(r.Context())
	role := req.Role
	if role == "" {
		role = store.RoleMember
	}

	fields := map[string]string{}
	handle, problem := validateAccountName(req.Username)
	if problem != "" {
		fields["username"] = problem
	}
	if len(req.Password) < 8 {
		fields["password"] = "Must be at least 8 characters."
	}
	if len(req.Password) > maxPassword {
		fields["password"] = "Must be at most 72 characters."
	}
	if role != store.RoleAdmin && role != store.RoleMember {
		fields["role"] = "Choose administrator or member."
	}
	if role == store.RoleAdmin && !actor.IsOwner() {
		fields["role"] = "Only the owner can create administrators."
	}
	if len(fields) > 0 {
		writeFieldErrors(w, fields)
		return
	}

	user, err := s.st.CreateUser(r.Context(), handle, req.Password, role)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.log.InfoContext(r.Context(), "account created",
		"username", user.Username, "role", user.Role, "by", actor.Username)

	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}
	if target.IsOwner() {
		writeError(w, http.StatusForbidden, "owner_protected",
			"The owner account cannot be deleted.")
		return
	}

	if err := s.st.DeleteUser(r.Context(), target.Username); err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.log.InfoContext(r.Context(), "account deleted",
		"username", target.Username, "by", userFrom(r.Context()).Username)

	w.WriteHeader(http.StatusNoContent)
}

type roleRequest struct {
	Role string `json:"role"`
}

// handleSetRole promotes and demotes accounts. Owner only, and ownership itself
// is still not transferable over HTTP — that stays a deliberate CLI step.
func (s *Server) handleSetRole(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}
	if target.IsOwner() {
		writeError(w, http.StatusForbidden, "owner_protected",
			"The owner's role cannot be changed here. Transfer ownership with "+
				"`phs-server user set-owner <username>` on the container.")
		return
	}

	var req roleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Role != store.RoleAdmin && req.Role != store.RoleMember {
		writeFieldErrors(w, map[string]string{"role": "Choose administrator or member."})
		return
	}

	updated, err := s.st.SetRole(r.Context(), target.Username, req.Role)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.log.InfoContext(r.Context(), "role changed",
		"username", updated.Username, "role", updated.Role, "by", userFrom(r.Context()).Username)

	writeJSON(w, http.StatusOK, updated)
}

type passwordResetRequest struct {
	Password string `json:"password"`
}

// handleResetPassword sets someone else's password, for when they lose it.
// Changing your own password is handled by /api/admin/password, which asks for
// the current one.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}
	var req passwordResetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Password) < 8 || len(req.Password) > maxPassword {
		writeFieldErrors(w, map[string]string{"password": "Must be between 8 and 72 characters."})
		return
	}

	if err := s.st.SetPassword(r.Context(), target.Username, req.Password); err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.log.InfoContext(r.Context(), "password reset",
		"username", target.Username, "by", userFrom(r.Context()).Username)

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

type verifiedRequest struct {
	Verified bool `json:"verified"`
}

func (s *Server) handleSetVerified(w http.ResponseWriter, r *http.Request) {
	target, ok := s.resolveTarget(w, r)
	if !ok {
		return
	}
	var req verifiedRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	actor := userFrom(r.Context())
	updated, err := s.st.SetVerified(r.Context(), target.Username, req.Verified, actor.Username)
	if err != nil {
		s.writeStoreError(w, r, err)
		return
	}
	s.log.InfoContext(r.Context(), "verified badge changed",
		"target", updated.Username, "verified", req.Verified, "by", actor.Username)

	writeJSON(w, http.StatusOK, updated)
}
