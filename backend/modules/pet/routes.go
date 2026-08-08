package pet

import (
	"database/sql"
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

func Routes(db *sql.DB) http.Handler {
	h := &handler{db: db}
	r := chi.NewRouter()

	r.Get("/", h.GetState)
	r.Post("/bathe", h.Bathe)
	r.Post("/breakfast", h.Breakfast)
	r.Post("/lunch", h.Lunch)
	r.Post("/play", h.Play)
	r.Post("/dinner", h.Dinner)
	r.Post("/shop/freeze", h.BuyFreeze)
	r.Post("/shop/rename", h.Rename)
	r.Post("/reset", h.Reset)

	return r
}
