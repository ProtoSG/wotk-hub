package auth

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"

	chi "github.com/go-chi/chi/v5"
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

// Register creates a new user account (max 2 users total — the first
// becomes admin, the second guest) and issues auth cookies.
//
// @Summary Register a new user
// @Description Public endpoint. Registration closes after 2 accounts exist; role is server-assigned (first=admin, second=guest).
// @Tags auth
// @Accept json
// @Produce json
// @Param body body registerRequest true "Registration details"
// @Success 201 {object} User
// @Failure 400 {object} httpx.APIError
// @Failure 403 {object} httpx.APIError "registration is closed"
// @Failure 409 {object} httpx.APIError "email already registered"
// @Router /auth/register [post]
func (h *handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		log.Printf("auth: hash password failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
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
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, registerLockKey); err != nil {
		log.Printf("auth: acquire registration lock failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		log.Printf("auth: count users failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if count >= maxUsers {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "registration is closed")
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
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "email already registered")
		return
	}
	if err != nil {
		log.Printf("auth: create user failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("auth: commit registration tx failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.issueTokens(w, userID, role); err != nil {
		log.Printf("auth: issue tokens failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, User{ID: userID, Name: req.Name, Email: req.Email, Role: role})
}

// Login validates credentials and issues auth cookies.
//
// @Summary Log in
// @Description Public endpoint. Validates email/password and sets access_token/refresh_token cookies.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Login credentials"
// @Success 200 {object} User
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError "invalid email or password"
// @Router /auth/login [post]
func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
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
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		log.Printf("auth: lookup user failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid email or password")
		return
	}

	if err := h.issueTokens(w, u.ID, u.Role); err != nil {
		log.Printf("auth: issue tokens failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

// Refresh rotates the refresh_token cookie for a new access/refresh pair.
//
// @Summary Refresh access token
// @Description Public endpoint (relies on the refresh_token cookie, not JWT auth). Atomically claims and rotates the refresh token.
// @Tags auth
// @Produce json
// @Success 200 {object} httpx.SuccessResponse
// @Failure 401 {object} httpx.APIError "unauthorized"
// @Router /auth/refresh [post]
func (h *handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
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
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		log.Printf("auth: claim refresh token failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var role string
	if err := h.db.QueryRow(`SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		log.Printf("auth: lookup user role failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := h.issueTokens(w, userID, role); err != nil {
		log.Printf("auth: issue tokens failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// Me returns the currently authenticated user.
//
// @Summary Get current user
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Success 200 {object} User
// @Failure 401 {object} httpx.APIError
// @Router /auth/me [get]
func (h *handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var u User
	err := h.db.QueryRow(`SELECT id, name, email, role FROM users WHERE id = $1`, userID).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		log.Printf("auth: lookup me failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

// Logout revokes the current refresh token and clears auth cookies.
//
// @Summary Log out
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Success 200 {object} httpx.SuccessResponse
// @Router /auth/logout [post]
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
	httpx.WriteSuccess(w, http.StatusOK)
}

// LogoutAll revokes every refresh token for the authenticated user (all
// sessions/devices) and clears auth cookies for the current one.
//
// @Summary Log out of all sessions
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Success 200 {object} httpx.SuccessResponse
// @Failure 401 {object} httpx.APIError
// @Router /auth/logout-all [post]
func (h *handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	if _, err := h.db.Exec(
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID,
	); err != nil {
		log.Printf("auth: revoke all on logout-all failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	clearAuthCookies(w, h.secure)
	httpx.WriteSuccess(w, http.StatusOK)
}

// DeleteUser deletes a user by id.
//
// @Summary Delete a user
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Param id path int true "User ID"
// @Success 200 {object} deleteUserResponse
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /auth/users/{id} [delete]
func (h *handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "id is required")
		return
	}

	result, err := h.db.Exec(`DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		log.Printf("auth: delete user failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "delete failed")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "user not found")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, deleteUserResponse{Deleted: true})
}

// ListUsers returns every user, including created_at (admin view).
//
// @Summary List users
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Success 200 {array} adminUserView
// @Router /auth/users [get]
func (h *handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, name, email, role, created_at FROM users ORDER BY id`)
	if err != nil {
		log.Printf("auth: list users failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "query failed")
		return
	}
	defer rows.Close()

	users := []adminUserView{}
	for rows.Next() {
		var u adminUserView
		var createdAt []byte
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &createdAt); err != nil {
			continue
		}
		u.CreatedAt = string(createdAt)
		users = append(users, u)
	}

	httpx.WriteJSON(w, http.StatusOK, users)
}
