package gym

import (
	"database/sql"
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

type handler struct {
	db *sql.DB
}

// Routes mounts the gym endpoints. Unlike finances.Routes there is no public
// split — nothing here is needed before login — so authentication is applied
// by the caller in main.go.
func Routes(db *sql.DB) http.Handler {
	h := &handler{db: db}
	r := chi.NewRouter()

	// Registered before /exercises/{id} would be, so "filters" is never
	// swallowed as an id param when the rest of the CRUD lands.
	r.Get("/exercises/filters", h.ExerciseFilters)
	r.Get("/exercises", h.ListExercises)
	r.Post("/exercises", h.CreateExercise)
	r.Put("/exercises/{id}", h.UpdateExercise)
	r.Put("/exercises/{id}/description", h.UpdateExerciseDescription)
	r.Delete("/exercises/{id}", h.DeleteExercise)
	r.Get("/exercises/{id}/last-sets", h.LastSets)

	// Same ordering reason: "active" must not be read as a session id.
	r.Get("/sessions/active", h.ActiveSession)
	r.Get("/sessions", h.ListSessions)
	r.Post("/sessions", h.CreateSession)
	r.Get("/sessions/{id}", h.GetSession)
	r.Put("/sessions/{id}", h.UpdateSession)
	r.Delete("/sessions/{id}", h.DeleteSession)
	r.Post("/sessions/{id}/finish", h.FinishSession)

	// "exercises" before "exercises/{id}" for the same ordering reason as above.
	r.Get("/progress/exercises", h.LoggedExercises)
	r.Get("/progress/exercises/{id}", h.ExerciseProgress)
	r.Get("/progress/summary", h.ProgressSummary)

	r.Get("/routines", h.ListRoutines)
	r.Post("/routines", h.CreateRoutine)
	r.Get("/routines/{id}", h.GetRoutine)
	r.Put("/routines/{id}", h.UpdateRoutine)
	r.Delete("/routines/{id}", h.DeleteRoutine)

	r.Post("/sessions/{id}/exercises", h.AddSessionExercise)
	r.Delete("/sessions/{id}/exercises/{exerciseId}", h.RemoveSessionExercise)
	r.Put("/sessions/{id}/exercises/{exerciseId}/sets", h.ReplaceSets)

	return r
}
