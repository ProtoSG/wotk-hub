package games

import (
	"database/sql"
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

type handler struct {
	db *sql.DB
}

func Routes(db *sql.DB) http.Handler {
	h := &handler{db: db}
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
