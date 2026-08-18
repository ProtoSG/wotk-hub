package permissions

import (
	"database/sql"
	"net/http"
	"workhub/middleware"

	chi "github.com/go-chi/chi/v5"
)

// Routes mounts the permissions endpoints. Expects to be mounted behind
// JWTAuth already (see main.go) — /mine needs a caller identity but not a
// specific role, /guest needs admin specifically, enforced per-route here
// the same way finances.Routes splits public/protected internally.
func Routes(db *sql.DB) http.Handler {
	h := &handler{db: db}
	r := chi.NewRouter()

	r.With(middleware.RequireRole("admin", "guest")).Get("/mine", h.ListMine)
	r.With(middleware.RequireRole("admin")).Get("/guest", h.ListGuest)
	r.With(middleware.RequireRole("admin")).Put("/guest", h.UpdateGuest)

	return r
}
