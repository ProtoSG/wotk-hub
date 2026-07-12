package auth

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is fixed higher than bcrypt.DefaultCost (10) since this app
// only ever has 1-2 users — the extra hashing time at login is negligible,
// and it's cheap insurance if the password_hash column ever leaks.
const bcryptCost = 12

// maxUsers caps self-registration at two accounts (the user + their partner).
// There is no admin UI to lift this — it's a permanent bootstrap-only limit.
const maxUsers = 2

// registerLockKey is an arbitrary fixed advisory-lock key. Register takes
// this lock for the duration of its transaction so concurrent registrations
// serialize on the count-check-then-insert sequence instead of racing (which
// could otherwise create two admins or exceed maxUsers).
const registerLockKey = 727384

// postgresUniqueViolation is the SQLSTATE code Postgres returns when an
// INSERT violates a UNIQUE constraint (e.g. the users.email index).
const postgresUniqueViolation = "23505"

// dummyPasswordHash is compared against on a login attempt for an email
// that doesn't exist, so the not-found path pays the same bcrypt cost as a
// real password check — otherwise the latency difference would leak
// whether an email is registered.
var dummyPasswordHash []byte

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing-parity"), bcryptCost)
	if err != nil {
		log.Fatalf("auth: generate dummy password hash failed: %v", err)
	}
	dummyPasswordHash = hash
}

func (h *handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		log.Printf("auth: hash password failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// The count-check-then-insert sequence below must be atomic against
	// concurrent registrations, or two simultaneous requests could both
	// read count=0 and both become admin, or both squeeze in under
	// maxUsers. pg_advisory_xact_lock serializes concurrent transactions
	// on this fixed key — the second one blocks until the first commits
	// (releasing the lock), so it sees the first's committed row count.
	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("auth: begin registration tx failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, registerLockKey); err != nil {
		log.Printf("auth: acquire registration lock failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		log.Printf("auth: count users failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if count >= maxUsers {
		httpx.WriteError(w, http.StatusForbidden, "registration is closed")
		return
	}
	// First registrant is admin, second is guest — server-decided, never
	// client-chosen.
	role := "admin"
	if count == 1 {
		role = "guest"
	}

	var userID int64
	err = tx.QueryRow(
		`INSERT INTO users (email, password_hash, name, role) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.Email, string(hash), req.Name, role,
	).Scan(&userID)
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == postgresUniqueViolation {
		httpx.WriteError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		log.Printf("auth: create user failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("auth: commit registration tx failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.issueTokens(w, userID, role); err != nil {
		log.Printf("auth: issue tokens failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, User{ID: userID, Name: req.Name, Email: req.Email, Role: role})
}

func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var u User
	var passwordHash string
	err := h.db.QueryRow(
		`SELECT id, name, email, role, password_hash FROM users WHERE email = $1`, req.Email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &passwordHash)
	if err == sql.ErrNoRows {
		// Compare against a fixed dummy hash so this path pays the same
		// bcrypt cost as the "email exists but password is wrong" path
		// below — otherwise the latency difference leaks whether an
		// email is registered.
		bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		httpx.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		log.Printf("auth: lookup user failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := h.issueTokens(w, u.ID, u.Role); err != nil {
		log.Printf("auth: issue tokens failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (h *handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	hash := hashToken(cookie.Value)

	// Atomically claim the token: only an UPDATE that actually flips
	// revoked_at from NULL to non-NULL succeeds, and only one of two
	// concurrent requests presenting the same token can win that race —
	// the loser gets sql.ErrNoRows, closing the replay window that a
	// separate SELECT-then-UPDATE would leave open.
	var userID int64
	err = h.db.QueryRow(
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		 RETURNING user_id`, hash,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		log.Printf("auth: claim refresh token failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var role string
	if err := h.db.QueryRow(`SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		log.Printf("auth: lookup user role failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.issueTokens(w, userID, role); err != nil {
		log.Printf("auth: issue tokens failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var u User
	err := h.db.QueryRow(`SELECT id, name, email, role FROM users WHERE id = $1`, userID).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		log.Printf("auth: lookup me failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (h *handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		hash := hashToken(cookie.Value)
		if _, err := h.db.Exec(
			`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash,
		); err != nil {
			log.Printf("auth: revoke on logout failed: %v", err)
		}
	}
	clearAuthCookies(w, h.secure)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
