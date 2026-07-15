package auth

import (
	"database/sql"
	"net/http"
	"workhub/middleware"

	chi "github.com/go-chi/chi/v5"
)

type handler struct {
	db     *sql.DB
	secret string
	secure bool
}

// Routes mounts the auth endpoints. Register/Login/Refresh are public (they
// establish or renew the session themselves); Me/Logout require an already
// valid access_token cookie, so they get their own internal JWTAuth group —
// chi doesn't let a parent r.Use exempt specific paths, so the public/
// protected split has to happen inside this router instead of by the caller.
func Routes(db *sql.DB, secret string, cookieSecure bool) http.Handler {
	h := &handler{db: db, secret: secret, secure: cookieSecure}
	r := chi.NewRouter()

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)

	r.Group(func(pr chi.Router) {
		pr.Use(middleware.JWTAuth(secret))
		pr.Get("/me", h.Me)
		pr.Post("/logout", h.Logout)
		pr.Delete("/users/{id}", h.DeleteUser)
		pr.Get("/users", h.ListUsers)
	})

	return r
}
