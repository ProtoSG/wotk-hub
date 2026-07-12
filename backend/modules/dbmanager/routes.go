package dbmanager

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/connect", Connect)
	// Tables, Schema and Relationships take connection credentials
	// (including the target DB password) in the request body rather than as
	// GET query parameters, so passwords never end up in URLs — see
	// connFromBody in handlers.go.
	r.Post("/tables", Tables)
	r.Post("/table/{name}/schema", Schema)
	r.Post("/query", Query)
	r.Post("/relationships", Relationships)
	return r
}
