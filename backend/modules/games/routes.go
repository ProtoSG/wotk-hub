package games

import (
	"database/sql"
	"net/http"
	"workhub/middleware"
	"workhub/shared/wshub"

	chi "github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type handler struct {
	db *sql.DB
	// VAPID* are empty when push notifications aren't configured (see
	// config.VAPIDPublicKey) — SubmitRiddleGuess checks before attempting
	// the partner-solved notification, same "feature quietly does nothing
	// without config" pattern as photoStorage/Minio.
	vapidPublicKey  string
	vapidPrivateKey string
	vapidSubject    string
	// hub fans emoji-movies/riddle session updates out to both partners'
	// live WS connections (see ServeWS and the broadcast* helpers in
	// handlers.go) — this replaces the old client-side polling. Owned by
	// this module (constructed here, not injected from main) since nothing
	// outside games needs it, same as sessionColumns/difficulties in
	// helpers.go being module-local constants rather than shared config.
	hub        *wshub.Hub
	wsUpgrader *websocket.Upgrader
}

func Routes(db *sql.DB, vapidPublicKey, vapidPrivateKey, vapidSubject, corsOrigins string) http.Handler {
	h := &handler{
		db:              db,
		vapidPublicKey:  vapidPublicKey,
		vapidPrivateKey: vapidPrivateKey,
		vapidSubject:    vapidSubject,
		hub:             wshub.New(),
		wsUpgrader:      wshub.NewUpgrader(middleware.ParseAllowedOrigins(corsOrigins)),
	}
	r := chi.NewRouter()

	r.Get("/emoji-movies", h.ListMovies)
	r.Post("/emoji-movies", h.CreateMovie)
	r.Get("/emoji-movies/random", h.RandomMovie)

	r.Post("/sessions", h.CreateSession)
	r.Get("/sessions/active", h.ActiveSessions)
	r.Get("/sessions/{id}", h.GetSession)
	r.Post("/sessions/{id}/join", h.JoinSession)
	r.Post("/sessions/{id}/guess", h.Guess)
	r.Post("/sessions/{id}/reveal", h.Reveal)

	r.Get("/riddle/today", h.GetRiddleToday)
	r.Get("/riddle/session", h.GetRiddleSession)
	r.Post("/riddle/guess", h.SubmitRiddleGuess)
	r.Get("/riddle/history", h.GetRiddleHistory)

	// Live-update channel for both games — replaces the old 6s poll. This
	// router is mounted behind JWTAuth in main.go (see the /api/games
	// group), so the access_token cookie is already validated and
	// middleware.UserFromContext already populated by the time ServeWS
	// runs, exactly like every other handler above.
	r.Get("/ws", h.ServeWS)

	return r
}
