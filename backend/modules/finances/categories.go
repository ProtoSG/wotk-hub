package finances

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"workhub/httpx"

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
func (h *handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, name, kind, label, created_at FROM categories`
	args := []any{}
	if kind := r.URL.Query().Get("kind"); kind != "" {
		args = append(args, kind)
		query += " WHERE kind = $1"
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

func (h *handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	row := h.db.QueryRow(
		`INSERT INTO categories (name, kind, label) VALUES ($1, $2, $3)
		 RETURNING id, name, kind, label, created_at`,
		req.Name, req.Kind, req.Label,
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

func (h *handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
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

	row := h.db.QueryRow(
		`UPDATE categories SET name = $1, kind = $2, label = $3 WHERE id = $4
		 RETURNING id, name, kind, label, created_at`,
		req.Name, req.Kind, req.Label, id,
	)
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
func (h *handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	var name string
	err = h.db.QueryRow(`SELECT name FROM categories WHERE id = $1`, id).Scan(&name)
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

	res, err := h.db.Exec(`DELETE FROM categories WHERE id = $1`, id)
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
