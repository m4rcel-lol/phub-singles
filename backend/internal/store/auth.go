package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role values. Exactly one owner exists at any time; the owner can only be
// changed from the command line, never through the HTTP API.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Rank orders the roles. Every permission check reduces to "may only act on an
// account of strictly lower rank", which is what keeps administrators away from
// each other and from the owner.
func Rank(role string) int {
	switch role {
	case RoleOwner:
		return 3
	case RoleAdmin:
		return 2
	case RoleMember:
		return 1
	default:
		return 0
	}
}

// ValidRole reports whether a string is a role the schema accepts.
func ValidRole(role string) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleMember:
		return true
	default:
		return false
	}
}

// User is an account. Only owners and admins may use the admin panel.
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	VerifiedAt   string `json:"verifiedAt,omitempty"`
	VerifiedBy   string `json:"verifiedBy,omitempty"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"createdAt"`
}

// IsAdmin reports whether the account may use the admin panel.
func (u User) IsAdmin() bool { return u.Role == RoleOwner || u.Role == RoleAdmin }

// IsOwner reports whether the account is the protected owner account.
func (u User) IsOwner() bool { return u.Role == RoleOwner }

const userColumns = `id, username, role, COALESCE(verified_at, ''), verified_by, password_hash, created_at`

// Same list, qualified for the sessions join.
const userColumnsJoined = `u.id, u.username, u.role, COALESCE(u.verified_at, ''), u.verified_by, u.password_hash, u.created_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Role, &u.VerifiedAt, &u.VerifiedBy,
		&u.PasswordHash, &u.CreatedAt)
	return u, err
}

// EnsureAdmin creates the bootstrap admin on first start. When force is true an
// existing account's password is reset to the supplied value, which is how an
// operator recovers access without touching the database by hand.
//
// It reports whether a password was written, so the caller can log it once.
func (s *Store) EnsureAdmin(ctx context.Context, username, password string, force bool) (created bool, reset bool, err error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return false, false, fmt.Errorf("%w: empty admin username", ErrInvalid)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, false, fmt.Errorf("count users: %w", err)
	}

	if count == 0 {
		if password == "" {
			return false, false, errors.New("no admin account exists and PHS_ADMIN_PASSWORD is not set")
		}
		hash, err := hashPassword(password)
		if err != nil {
			return false, false, err
		}
		// The very first account is the owner, and it owns the page.
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO users (username, password_hash, role, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			username, hash, RoleOwner, nowUTC(), nowUTC())
		if err != nil {
			return false, false, fmt.Errorf("create admin: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return false, false, fmt.Errorf("create admin id: %w", err)
		}
		if err := s.claimProfile(ctx, id); err != nil {
			return false, false, err
		}
		return true, false, nil
	}

	if !force {
		return false, false, nil
	}
	hash, err := hashPassword(password)
	if err != nil {
		return false, false, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = ?, password_hash = ?, updated_at = ?
		 WHERE id = (SELECT id FROM users WHERE role = ? ORDER BY id LIMIT 1)`,
		username, hash, nowUTC(), RoleOwner)
	if err != nil {
		return false, false, fmt.Errorf("reset admin password: %w", err)
	}
	n, _ := res.RowsAffected()
	return false, n > 0, nil
}

// Authenticate verifies a username/password pair in constant time with respect
// to whether the account exists: a missing user still costs one bcrypt compare.
func (s *Store) Authenticate(ctx context.Context, username, password string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = ? COLLATE NOCASE`,
		strings.TrimSpace(username)))
	if errors.Is(err, sql.ErrNoRows) {
		// Dummy compare keeps the response time of unknown users comparable.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return User{}, ErrNotFound
	}
	return u, nil
}

// ChangePassword updates the password of an account after re-checking the old one.
func (s *Store) ChangePassword(ctx context.Context, userID int64, current, next string) error {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)); err != nil {
		return ErrNotFound
	}
	newHash, err := hashPassword(next)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		newHash, nowUTC(), userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	// A password change invalidates every other session.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("clear sessions: %w", err)
	}
	return nil
}

// CreateSession stores the hash of a freshly minted session token.
func (s *Store) CreateSession(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	expires := time.Now().UTC().Add(ttl).Format("2006-01-02T15:04:05Z")
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		HashToken(token), userID, expires, nowUTC()); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// UserBySession resolves a session token to its owner, rejecting expired rows.
func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumnsJoined+`
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`,
		HashToken(token), nowUTC()))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load session: %w", err)
	}
	return u, nil
}

// DeleteSession revokes a single session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, HashToken(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// PurgeExpiredSessions drops stale rows; called periodically by the server.
func (s *Store) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, nowUTC())
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// HashToken maps a session token to the value stored in the database.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("%w: password must be at least 8 characters", ErrInvalid)
	}
	if len(password) > 72 { // bcrypt truncates beyond 72 bytes; reject instead.
		return "", fmt.Errorf("%w: password must be at most 72 characters", ErrInvalid)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// dummyHash is a valid bcrypt hash of a random value, used to equalise timing.
var dummyHash = []byte("$2a$12$eImiTXuWVxfM37uY4JANjQ.hUkI3wjNL8/wS/8UcHrXlPKGZ2A6ye")

// claimProfile points the single profile at an account when it has no owner.
func (s *Store) claimProfile(ctx context.Context, userID int64) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE profile SET user_id = ? WHERE id = 1 AND user_id IS NULL`, userID); err != nil {
		return fmt.Errorf("assign profile owner: %w", err)
	}
	return nil
}

// Users lists every account, owner first, then admins, then members.
func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+userColumns+` FROM users
		 ORDER BY CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, username`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0, 8)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// User loads one account by username.
func (s *Store) User(ctx context.Context, username string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = ? COLLATE NOCASE`,
		strings.TrimSpace(username)))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	return u, nil
}

// CreateUser adds an account. Creating a second owner is rejected: ownership is
// transferred with SetOwner, never duplicated.
func (s *Store) CreateUser(ctx context.Context, username, password, role string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, fmt.Errorf("%w: username is required", ErrInvalid)
	}
	if !ValidRole(role) {
		return User{}, fmt.Errorf("%w: unknown role %q", ErrInvalid, role)
	}
	if role == RoleOwner {
		return User{}, fmt.Errorf("%w: use set-owner to transfer ownership", ErrInvalid)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}

	if _, err := s.User(ctx, username); err == nil {
		return User{}, fmt.Errorf("%w: user %q already exists", ErrConflict, username)
	} else if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		username, hash, role, nowUTC(), nowUTC()); err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return s.User(ctx, username)
}

// DeleteUser removes an account and its sessions. The owner cannot be deleted.
func (s *Store) DeleteUser(ctx context.Context, username string) error {
	u, err := s.User(ctx, username)
	if err != nil {
		return err
	}
	if u.IsOwner() {
		return fmt.Errorf("%w: the owner account cannot be deleted", ErrInvalid)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, u.ID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// SetRole changes an account's role. Ownership is never granted here; demoting
// the owner is refused so the site always has exactly one.
func (s *Store) SetRole(ctx context.Context, username, role string) (User, error) {
	if !ValidRole(role) || role == RoleOwner {
		return User{}, fmt.Errorf("%w: role must be admin or member", ErrInvalid)
	}
	u, err := s.User(ctx, username)
	if err != nil {
		return User{}, err
	}
	if u.IsOwner() {
		return User{}, fmt.Errorf("%w: transfer ownership before demoting the owner", ErrInvalid)
	}
	if u.Role == role {
		return u, nil
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`, role, nowUTC(), u.ID); err != nil {
		return User{}, fmt.Errorf("set role: %w", err)
	}
	return s.User(ctx, username)
}

// SetOwner transfers ownership: the new owner takes the role and the page, and
// the previous owner is demoted to admin. Command-line only by design.
func (s *Store) SetOwner(ctx context.Context, username string) (User, error) {
	target, err := s.User(ctx, username)
	if err != nil {
		return User{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin ownership transfer: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET role = ?, updated_at = ? WHERE role = ? AND id <> ?`,
		RoleAdmin, nowUTC(), RoleOwner, target.ID); err != nil {
		return User{}, fmt.Errorf("demote previous owner: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`,
		RoleOwner, nowUTC(), target.ID); err != nil {
		return User{}, fmt.Errorf("promote new owner: %w", err)
	}
	// The page follows ownership.
	if _, err := tx.ExecContext(ctx, `UPDATE profile SET user_id = ? WHERE id = 1`, target.ID); err != nil {
		return User{}, fmt.Errorf("reassign profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit ownership transfer: %w", err)
	}
	return s.User(ctx, username)
}

// SetVerified grants or revokes the manual Verified badge. `by` records who did
// it, for the audit trail shown in the admin panel.
func (s *Store) SetVerified(ctx context.Context, username string, verified bool, by string) (User, error) {
	u, err := s.User(ctx, username)
	if err != nil {
		return User{}, err
	}

	if verified {
		_, err = s.db.ExecContext(ctx,
			`UPDATE users SET verified_at = ?, verified_by = ?, updated_at = ? WHERE id = ?`,
			nowUTC(), by, nowUTC(), u.ID)
	} else {
		_, err = s.db.ExecContext(ctx,
			`UPDATE users SET verified_at = NULL, verified_by = '', updated_at = ? WHERE id = ?`,
			nowUTC(), u.ID)
	}
	if err != nil {
		return User{}, fmt.Errorf("set verified: %w", err)
	}
	return s.User(ctx, username)
}

// SetPassword replaces an account's password without knowing the old one. It is
// reachable from the CLI only; the HTTP path is ChangePassword.
func (s *Store) SetPassword(ctx context.Context, username, password string) error {
	u, err := s.User(ctx, username)
	if err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, nowUTC(), u.ID); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, u.ID); err != nil {
		return fmt.Errorf("clear sessions: %w", err)
	}
	return nil
}
