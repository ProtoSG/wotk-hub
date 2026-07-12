package couple

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

	r.Get("/dates", h.ListDates)
	r.Post("/dates", h.CreateDate)
	r.Put("/dates/{id}", h.UpdateDate)
	r.Delete("/dates/{id}", h.DeleteDate)

	return r
}
