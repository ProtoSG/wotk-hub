package permissions

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"
	"workhub/shared/team"
)

type handler struct {
	db *sql.DB
}

// ListMine returns the caller's own accessible module keys. Admin always
// gets every module in AllModules — this table only ever gates the guest
// account, admin is never restricted by it. Fetched once at app load
// alongside /auth/me, so the frontend can build its nav/routes from real
// data instead of a hardcoded roles array.
//
// @Summary List the caller's accessible modules
// @Tags permissions
// @Produce json
// @Security CookieAuth
// @Success 200 {object} minePermissionsResponse
// @Failure 401 {object} httpx.APIError
// @Router /permissions/mine [get]
func (h *handler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	if role == "admin" {
		httpx.WriteJSON(w, http.StatusOK, minePermissionsResponse{Modules: AllModules})
		return
	}

	rows, err := h.db.Query(
		`SELECT module FROM module_permissions WHERE user_id = $1 AND enabled`, userID)
	if err != nil {
		log.Printf("permissions: list mine failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	modules := []string{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			log.Printf("permissions: scan mine failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		modules = append(modules, m)
	}
	httpx.WriteJSON(w, http.StatusOK, minePermissionsResponse{Modules: modules})
}

// ListGuest returns every grantable module with its current state for the
// guest account, so Configuración can render one row per module regardless
// of whether a row already exists for it (no row = not yet toggled = false).
//
// @Summary List the guest account's module grants
// @Tags permissions
// @Produce json
// @Security CookieAuth
// @Success 200 {object} listGuestPermissionsResponse
// @Failure 404 {object} httpx.APIError "no guest account exists yet"
// @Router /permissions/guest [get]
func (h *handler) ListGuest(w http.ResponseWriter, r *http.Request) {
	guestID, err := team.ResolveGuestID(h.db)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no guest account exists yet")
		return
	}
	if err != nil {
		log.Printf("permissions: resolve guest failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	enabled := map[string]bool{}
	rows, err := h.db.Query(`SELECT module, enabled FROM module_permissions WHERE user_id = $1`, guestID)
	if err != nil {
		log.Printf("permissions: list guest failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var m string
		var e bool
		if err := rows.Scan(&m, &e); err != nil {
			log.Printf("permissions: scan guest failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		enabled[m] = e
	}

	resp := listGuestPermissionsResponse{Modules: make([]ModulePermission, 0, len(AllModules))}
	for _, m := range AllModules {
		resp.Modules = append(resp.Modules, ModulePermission{Module: m, Enabled: enabled[m]})
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// UpdateGuest upserts the submitted module -> enabled pairs for the guest
// account. Unknown module keys are rejected rather than silently accepted —
// AllModules is the only valid set, DB Manager/Configuración included on
// neither side of that check by construction (see AllModules' comment).
//
// @Summary Update the guest account's module grants
// @Tags permissions
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body updateGuestRequest true "Module grants to change"
// @Success 200 {object} listGuestPermissionsResponse
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError "no guest account exists yet"
// @Router /permissions/guest [put]
func (h *handler) UpdateGuest(w http.ResponseWriter, r *http.Request) {
	adminID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req updateGuestRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	for m := range req.Modules {
		if !isValidModule(m) {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid module: "+m)
			return
		}
	}

	guestID, err := team.ResolveGuestID(h.db)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "no guest account exists yet")
		return
	}
	if err != nil {
		log.Printf("permissions: resolve guest failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("permissions: update guest begin failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	for module, enabled := range req.Modules {
		if _, err := tx.Exec(
			`INSERT INTO module_permissions (user_id, module, enabled, updated_at, updated_by)
			 VALUES ($1, $2, $3, now(), $4)
			 ON CONFLICT (user_id, module)
			 DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now(), updated_by = EXCLUDED.updated_by`,
			guestID, module, enabled, adminID,
		); err != nil {
			log.Printf("permissions: update guest write failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("permissions: update guest commit failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	h.ListGuest(w, r)
}
