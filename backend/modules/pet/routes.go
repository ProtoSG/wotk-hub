package pet

import (
	"database/sql"
	"net/http"
	"workhub/middleware"
	"workhub/storage"

	chi "github.com/go-chi/chi/v5"
)

// Routes returns the router for the shared couple pet. opencodeAPIKey and
// opencodeModel back the /chat route (see chat.go); elevenLabsAPIKey and
// elevenLabsVoiceID back the /speak route (see speak.go). photoStorage backs
// the photo routes (photos.go); it may be nil, in which case photo handlers
// return 503 — same pattern couple.Routes uses for its own photo storage.
func Routes(db *sql.DB, opencodeAPIKey, opencodeModel, elevenLabsAPIKey, elevenLabsVoiceID string, photoStorage *storage.Client) http.Handler {
	h := &handler{
		db:                db,
		opencodeAPIKey:    opencodeAPIKey,
		opencodeModel:     opencodeModel,
		elevenLabsAPIKey:  elevenLabsAPIKey,
		elevenLabsVoiceID: elevenLabsVoiceID,
		storage:           photoStorage,
	}
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
	r.Post("/chat", h.Chat)
	r.Post("/speak", h.Speak)

	// Photo routes — storage may be nil (see above). Delete/clear admin-only,
	// same reasoning as couple.Routes' video delete: the module as a whole
	// is mounted for both admin and guest, so destructive actions need their
	// own explicit role gate.
	r.Post("/photos", h.UploadPhoto)
	r.Get("/photos", h.ListPetPhotos)
	r.With(middleware.RequireRole("admin")).Delete("/photos/{photoId}", h.DeletePetPhoto)
	r.With(middleware.RequireRole("admin")).Delete("/photos", h.ClearPetPhotos)

	return r
}
