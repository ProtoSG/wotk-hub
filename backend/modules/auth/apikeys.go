package auth

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"

	chi "github.com/go-chi/chi/v5"
)

// CreateAPIKey mints a new long-lived API key for the caller, scoped to
// their own user_id. Meant for external automation (e.g. an iOS/macOS
// Shortcut) that can't do an interactive login and instead sends
// "Authorization: Bearer <key>" on every request — see
// middleware.RequireAuth.
//
// The raw key is returned ONLY in this response. Only its SHA-256 hash is
// ever persisted, so if it's lost it cannot be recovered — a new key has to
// be minted and the old one revoked.
//
// @Summary Create an API key
// @Description Requires an active login session (cookie), not another API key — you can't mint a key using a key. The "key" field is shown only once and cannot be retrieved again.
// @Tags auth
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body createAPIKeyRequest true "Key details"
// @Success 201 {object} apiKeyCreated
// @Failure 401 {object} httpx.APIError
// @Failure 500 {object} httpx.APIError
// @Router /auth/keys [post]
func (h *handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	var req createAPIKeyRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	raw, hash, err := newAPIKey()
	if err != nil {
		log.Printf("auth: generate api key failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var resp apiKeyCreated
	var createdAt []byte
	err = h.db.QueryRow(
		`INSERT INTO api_keys (user_id, name, key_hash) VALUES ($1, $2, $3) RETURNING id, created_at`,
		userID, req.Name, hash,
	).Scan(&resp.ID, &createdAt)
	if err != nil {
		log.Printf("auth: create api key failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	resp.Name = req.Name
	resp.Key = raw
	resp.CreatedAt = string(createdAt)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// ListAPIKeys returns the caller's own API keys — never the hash or raw key,
// only enough to identify and manage them.
//
// @Summary List API keys
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Success 200 {array} apiKeyView
// @Failure 401 {object} httpx.APIError
// @Router /auth/keys [get]
func (h *handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	rows, err := h.db.Query(
		`SELECT id, name, created_at, last_used_at, revoked_at FROM api_keys
		 WHERE user_id = $1 ORDER BY id`, userID,
	)
	if err != nil {
		log.Printf("auth: list api keys failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "query failed")
		return
	}
	defer rows.Close()

	keys := []apiKeyView{}
	for rows.Next() {
		var k apiKeyView
		var createdAt []byte
		var lastUsedAt, revokedAt sql.NullString
		if err := rows.Scan(&k.ID, &k.Name, &createdAt, &lastUsedAt, &revokedAt); err != nil {
			continue
		}
		k.CreatedAt = string(createdAt)
		if lastUsedAt.Valid {
			k.LastUsedAt = &lastUsedAt.String
		}
		if revokedAt.Valid {
			k.RevokedAt = &revokedAt.String
		}
		keys = append(keys, k)
	}

	httpx.WriteJSON(w, http.StatusOK, keys)
}

// RevokeAPIKey revokes one of the caller's own API keys by id. Scoped to
// user_id so a user can't revoke someone else's key.
//
// @Summary Revoke an API key
// @Tags auth
// @Produce json
// @Security CookieAuth
// @Param id path int true "API key ID"
// @Success 200 {object} revokeAPIKeyResponse
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /auth/keys/{id} [delete]
func (h *handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "id is required")
		return
	}

	result, err := h.db.Exec(
		`UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		id, userID,
	)
	if err != nil {
		log.Printf("auth: revoke api key failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "revoke failed")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "api key not found")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, revokeAPIKeyResponse{Revoked: true})
}
