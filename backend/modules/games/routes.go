package games

import (
	"database/sql"
	"net/http"

	chi "github.com/go-chi/chi/v5"
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
}

func Routes(db *sql.DB, vapidPublicKey, vapidPrivateKey, vapidSubject string) http.Handler {
	h := &handler{db: db, vapidPublicKey: vapidPublicKey, vapidPrivateKey: vapidPrivateKey, vapidSubject: vapidSubject}
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

	return r
}
