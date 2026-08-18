package finances

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"workhub/httpx"
	"workhub/middleware"

	"github.com/lib/pq"
)

// categoryUniqueViolation mirrors auth.postgresUniqueViolation — same
// Postgres error code, redefined locally since the auth constant is
// unexported outside its package.
const categoryUniqueViolation = "23505"

func scanCategory(row interface{ Scan(...any) error }) (Category, error) {
	var c Category
	var createdAt time.Time
	err := row.Scan(&c.ID, &c.Name, &c.Kind, &c.Label, &createdAt)
	if err != nil {
		return c, err
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	return c, nil
}

// categoryExists checks a category with the given name and kind exists.
// transactions/subscriptions/budgets.category are plain TEXT with no FK
// (same tradeoff as the rest of the finance schema), so this is the write-
// time validation that stands in for one.
func (h *handler) categoryExists(name, kind string) error {
	var got int64
	return h.db.QueryRow(`SELECT id FROM categories WHERE name = $1 AND kind = $2`, name, kind).Scan(&got)
}

// categoryInUse reports whether any transaction, subscription, or budget
// still references name — the FK check a real foreign key would give for
// free, done manually since category columns are plain TEXT.
func (h *handler) categoryInUse(name string) (bool, error) {
	var inUse bool
	err := h.db.QueryRow(
		`SELECT
		   EXISTS (SELECT 1 FROM transactions WHERE category = $1 AND deleted_at IS NULL)
		   OR EXISTS (SELECT 1 FROM subscriptions WHERE category = $1)
		   OR EXISTS (SELECT 1 FROM budgets WHERE category = $1)`,
		name,
	).Scan(&inUse)
	return inUse, err
}

// ListCategories returns all categories, optionally filtered by ?kind=.
//
// @Summary List categories
// @Tags finances
// @Produce json
// @Param kind query string false "Filter by kind (income, expense)"
// @Success 200 {object} listCategoriesResponse
// @Router /finances/categories [get]
func (h *handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	// Deliberately public (see routes.go) — unauthenticated callers (e.g. a
	// category picker on the pre-login onboarding flow) still get a result,
	// just the shared/ownerless defaults rather than any user's private
	// ones. middleware.UserFromContext returning !ok here means "no cookie",
	// not an error.
	userID, role, ok := middleware.UserFromContext(r.Context())

	query := `SELECT id, name, kind, label, created_at FROM categories WHERE true`
	args := []any{}
	if kind := r.URL.Query().Get("kind"); kind != "" {
		args = append(args, kind)
		query += " AND kind = $" + itoa(len(args))
	}
	// Authenticated non-admin sees their own categories plus the shared,
	// ownerless defaults (created_by IS NULL — the seeded taxonomy from
	// migrate.go, or any pre-scoping legacy row) — not every other user's
	// private ones. Unauthenticated and admin both fall through: admin sees
	// everything (existing scopeToOwner convention), unauthenticated sees
	// only what an anonymous caller reasonably should — the shared defaults.
	switch {
	case ok && role != "admin":
		args = append(args, userID)
		query += " AND (created_by = $" + itoa(len(args)) + " OR created_by IS NULL)"
	case !ok:
		query += " AND created_by IS NULL"
	}
	query += " ORDER BY kind, label"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		log.Printf("finances: list categories failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			log.Printf("finances: scan category failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		categories = append(categories, c)
	}
	httpx.WriteJSON(w, http.StatusOK, listCategoriesResponse{Categories: categories})
}

// @Summary Create a category
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body categoryRequest true "Category details"
// @Success 201 {object} Category
// @Failure 400 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError "category already exists"
// @Router /finances/categories [post]
func (h *handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req categoryRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	// The DB's own unique index only catches a caller re-creating one of
	// their OWN categories (idx_categories_name_kind_owned, scoped by
	// created_by) — it doesn't stop a caller from shadowing a shared/seeded
	// default (created_by IS NULL) with a private copy of the same name,
	// since those live in a separate partial index on purpose (see
	// migrate.go's comment on idx_categories_name_kind_shared). Reject that
	// case explicitly here instead: a name the caller can already see in
	// their own list (the shared defaults) should read as "already exists",
	// not silently produce a second, confusing entry with the same name.
	var sharedExists bool
	if err := h.db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM categories WHERE name = $1 AND kind = $2 AND created_by IS NULL)`,
		req.Name, req.Kind,
	).Scan(&sharedExists); err != nil {
		log.Printf("finances: create category shared-name check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if sharedExists {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "category already exists")
		return
	}

	row := h.db.QueryRow(
		`INSERT INTO categories (name, kind, label, created_by) VALUES ($1, $2, $3, $4)
		 RETURNING id, name, kind, label, created_at`,
		req.Name, req.Kind, req.Label, userID,
	)
	c, err := scanCategory(row)
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == categoryUniqueViolation {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "category already exists")
		return
	}
	if err != nil {
		log.Printf("finances: create category failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

// @Summary Update a category
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Category ID"
// @Param body body categoryRequest true "Category details"
// @Success 200 {object} Category
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError "category already exists"
// @Router /finances/categories/{id} [put]
func (h *handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req categoryRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	// Same shadowing guard as CreateCategory: renaming a non-admin's own
	// category to match a shared default's name would otherwise sail past
	// the DB's owned-only partial index and produce a confusing duplicate.
	// Doesn't apply to admin — admin edits the shared defaults directly
	// (scopeToOwner leaves them unscoped), there's no separate "own" pool to
	// collide with.
	if role != "admin" {
		var sharedExists bool
		if err := h.db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM categories WHERE name = $1 AND kind = $2 AND created_by IS NULL)`,
			req.Name, req.Kind,
		).Scan(&sharedExists); err != nil {
			log.Printf("finances: update category shared-name check failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		if sharedExists {
			httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "category already exists")
			return
		}
	}

	// scopeToOwner's "AND created_by = $N" naturally excludes NULL-owner
	// (seeded default) categories for non-admin too — a guest can read them
	// but not mutate them, only admin can.
	query, args := scopeToOwner(`UPDATE categories SET name = $1, kind = $2, label = $3 WHERE id = $4`,
		[]any{req.Name, req.Kind, req.Label, id}, role, userID)
	query += ` RETURNING id, name, kind, label, created_at`
	row := h.db.QueryRow(query, args...)
	c, err := scanCategory(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "category not found")
		return
	}
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == categoryUniqueViolation {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "category already exists")
		return
	}
	if err != nil {
		log.Printf("finances: update category failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c)
}

// DeleteCategory refuses to delete a category still referenced by a
// transaction, subscription, or budget (see categoryInUse) — the FK check
// the plan requires, enforced manually since those columns have no real
// foreign key to categories.
//
// @Summary Delete a category
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param id path int true "Category ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError "category is in use"
// @Router /finances/categories/{id} [delete]
func (h *handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	// Ownership-gated lookup: 404s (not just a plain "not found") for a
	// guest trying to delete another user's or a shared default category —
	// see UpdateCategory's comment on why scopeToOwner also gates NULL-owner
	// rows for non-admin.
	lookupQuery, lookupArgs := scopeToOwner(`SELECT name FROM categories WHERE id = $1`, []any{id}, role, userID)
	var name string
	err = h.db.QueryRow(lookupQuery, lookupArgs...).Scan(&name)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "category not found")
		return
	}
	if err != nil {
		log.Printf("finances: delete category lookup failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	inUse, err := h.categoryInUse(name)
	if err != nil {
		log.Printf("finances: delete category usage check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if inUse {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "category is in use and cannot be deleted")
		return
	}

	deleteQuery, deleteArgs := scopeToOwner(`DELETE FROM categories WHERE id = $1`, []any{id}, role, userID)
	res, err := h.db.Exec(deleteQuery, deleteArgs...)
	if err != nil {
		log.Printf("finances: delete category failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "category not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}
