package cli

import (
	"database/sql"
	"net/http"

	"workhub/httpx"
	"workhub/modules/auth"

	"github.com/go-chi/chi/v5"
)

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
//
// @Summary Get the CLI-token user
// @Description Only mounted when CLI_TOKEN is set. Intended for the workhubctl CLI tool.
// @Tags cli
// @Produce json
// @Security BearerAuth
// @Success 200 {object} auth.User
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /cli/me [get]
func (h *handler) Me(w http.ResponseWriter, r *http.Request) {
	var u auth.User
	err := h.db.QueryRow(`SELECT id, name, email, role FROM users WHERE id = $1`, h.cliUserID).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "cli user not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "database error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}
