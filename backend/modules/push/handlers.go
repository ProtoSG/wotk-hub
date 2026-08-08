package push

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"workhub/httpx"
	"workhub/middleware"
)

type handler struct {
	db          *sql.DB
	vapidPublic string
}

// VAPIDPublicKey hands the browser the public half of the keypair so it can
// call PushManager.subscribe({applicationServerKey: publicKey}) — the
// private key never leaves the server.
func (h *handler) VAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, vapidKeyResponse{PublicKey: h.vapidPublic})
}

// Subscribe saves (or refreshes) a browser's push subscription for the
// logged-in user. Idempotent per (user, endpoint) — see the migration's
// UNIQUE constraint — so re-subscribing the same device just updates keys
// instead of piling up duplicate rows the reminder job would double-send to.
func (h *handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req subscribeRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "endpoint and keys are required")
		return
	}
	if _, err := h.db.Exec(
		`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, endpoint) DO UPDATE SET p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth`,
		userID, endpoint, req.Keys.P256dh, req.Keys.Auth,
	); err != nil {
		log.Printf("push: subscribe failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// Unsubscribe removes a device's subscription — called when the user turns
// reminders off, or the frontend detects the browser's own subscription was
// cleared.
func (h *handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req unsubscribeRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if _, err := h.db.Exec(
		`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`,
		userID, req.Endpoint,
	); err != nil {
		log.Printf("push: unsubscribe failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}
