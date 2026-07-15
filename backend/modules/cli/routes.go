package cli

import (
	"database/sql"
	"net/http"

	"workhub/httpx"

	"github.com/go-chi/chi/v5"
)

// handler holds dependencies for CLI routes.
type handler struct {
	db        *sql.DB
	cliUserID int64 // default user ID for CLI token auth
}

// Routes mounts CLI routes behind CLITokenAuth. These are admin-only
// endpoints intended for the workhubctl CLI tool.
func Routes(db *sql.DB, cliUserID int64, cliToken string, cliAuthMiddleware func(http.Handler) http.Handler) http.Handler {
	h := &handler{db: db, cliUserID: cliUserID}
	r := chi.NewRouter()
	r.Use(cliAuthMiddleware)
	r.Get("/me", h.Me)
	return r
}

// Me returns the user associated with the CLI token.
func (h *handler) Me(w http.ResponseWriter, r *http.Request) {
	var id int64
	var name, email, role string
	err := h.db.QueryRow(`SELECT id, name, email, role FROM users WHERE id = $1`, h.cliUserID).Scan(&id, &name, &email, &role)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, "cli user not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "database error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"id":    id,
		"name":  name,
		"email": email,
		"role":  role,
	})
}
