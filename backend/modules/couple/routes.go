package couple

import (
	"database/sql"
	"net/http"
	"workhub/storage"

	chi "github.com/go-chi/chi/v5"
)

type handler struct {
	db *sql.DB
	// storage is nil when the photo feature is disabled (MINIO_ENDPOINT
	// unset) — every photo handler checks for nil before touching it and
	// returns 503 instead of reaching a nil pointer. See main.go wiring.
	storage *storage.Client
}

// Routes mounts the couple module's routes. photoStorage may be nil, which
// disables the photo endpoints (they respond 503) without disabling the rest
// of the module — mirrors how finances.Routes takes extra args beyond db.
func Routes(db *sql.DB, photoStorage *storage.Client) http.Handler {
	h := &handler{db: db, storage: photoStorage}
	r := chi.NewRouter()

	r.Get("/dates", h.ListDates)
	r.Post("/dates", h.CreateDate)
	r.Put("/dates/{id}", h.UpdateDate)
	r.Delete("/dates/{id}", h.DeleteDate)

	r.Post("/dates/{id}/photos", h.UploadPhoto)
	r.Get("/dates/{id}/photos", h.ListPhotos)
	r.Put("/dates/{id}/photos/{photoId}", h.UpdatePhoto)
	r.Delete("/dates/{id}/photos/{photoId}", h.DeletePhoto)
	r.Get("/photos", h.ListGalleryPhotos)

	return r
}
