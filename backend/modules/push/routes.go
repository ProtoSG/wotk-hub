package push

import (
	"database/sql"
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

func Routes(db *sql.DB, vapidPublicKey string) http.Handler {
	h := &handler{db: db, vapidPublic: vapidPublicKey}
	r := chi.NewRouter()

	r.Get("/vapid-key", h.VAPIDPublicKey)
	r.Post("/subscribe", h.Subscribe)
	r.Post("/unsubscribe", h.Unsubscribe)

	return r
}
