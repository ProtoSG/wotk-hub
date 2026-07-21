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

	return r
}
