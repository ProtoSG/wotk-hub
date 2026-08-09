package pet

import (
	"database/sql"
	"net/http"

	chi "github.com/go-chi/chi/v5"
)

// Routes returns the router for the shared couple pet. opencodeAPIKey and
// opencodeModel back the /chat route (see chat.go); elevenLabsAPIKey and
// elevenLabsVoiceID back the /speak route (see speak.go). An empty key
// doesn't change what's mounted here (unlike ytdlp's public route or push's
// whole module) — Chat/Speak themselves 503 per-request when unconfigured,
// since both live inside this existing pet router rather than a standalone
// module with a mount-time on/off switch.
func Routes(db *sql.DB, opencodeAPIKey, opencodeModel, elevenLabsAPIKey, elevenLabsVoiceID string) http.Handler {
	h := &handler{
		db:                db,
		opencodeAPIKey:    opencodeAPIKey,
		opencodeModel:     opencodeModel,
		elevenLabsAPIKey:  elevenLabsAPIKey,
		elevenLabsVoiceID: elevenLabsVoiceID,
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

	return r
}
