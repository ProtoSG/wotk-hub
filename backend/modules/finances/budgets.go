package finances

import (
	"database/sql"
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"

	chi "github.com/go-chi/chi/v5"
)

// ListBudgets scopes the budget-vs-spent join by created_by for non-admin
// roles (see scopeToOwner), so a guest's "spent" figures only reflect their
// own transactions. Admins see everything unscoped.
// @Summary List budgets
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param month query string false "Month, YYYY-MM (defaults to current month)"
// @Success 200 {object} listBudgetsResponse
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Router /finances/budgets [get]
func (h *handler) ListBudgets(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	start, end, err := monthRange(r.URL.Query().Get("month"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	// Both budgets and transactions now have their own created_by, so the
	// join condition (not scopeToOwner's plain "AND created_by = $N", which
	// would be ambiguous across two tables) is what keeps "spent" scoped to
	// the budget owner's own transactions in that category — not the whole
	// household's.
	query := `SELECT b.id, b.category, b.monthly_limit_cents, COALESCE(SUM(t.amount_cents), 0) AS spent
		 FROM budgets b
		 LEFT JOIN transactions t
		   ON t.category = b.category AND t.type = 'expense' AND t.deleted_at IS NULL
		  AND t.occurred_on >= $1 AND t.occurred_on < $2
		  AND t.created_by = b.created_by`
	args := []any{start, end}
	if role != "admin" {
		args = append(args, userID)
		query += " WHERE b.created_by = $" + itoa(len(args))
	}
	rows, err := h.db.Query(query+" GROUP BY b.id ORDER BY b.category", args...)
	if err != nil {
		log.Printf("finances: list budgets failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	budgets := []Budget{}
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.ID, &b.Category, &b.MonthlyLimitCents, &b.SpentCents); err != nil {
			log.Printf("finances: scan budget failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		budgets = append(budgets, b)
	}
	httpx.WriteJSON(w, http.StatusOK, listBudgetsResponse{Budgets: budgets})
}

// UpsertBudget creates or updates the monthly limit for a category.
//
// @Summary Upsert a budget
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param category path string true "Expense category"
// @Param body body budgetRequest true "Budget details"
// @Success 200 {object} Budget
// @Failure 400 {object} httpx.APIError
// @Router /finances/budgets/{category} [put]
func (h *handler) UpsertBudget(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	category := chi.URLParam(r, "category")
	var req budgetRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(category); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	if err := h.categoryExists(category, "expense"); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid category: "+category)
		return
	} else if err != nil {
		log.Printf("finances: upsert budget category check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	var b Budget
	// ON CONFLICT (category, created_by) matches the budgets_category_created_by_key
	// constraint — this upsert can only ever create or update the caller's own
	// row for this category, never another user's.
	err := h.db.QueryRow(
		`INSERT INTO budgets (category, monthly_limit_cents, created_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (category, created_by) DO UPDATE SET monthly_limit_cents = EXCLUDED.monthly_limit_cents
		 RETURNING id, category, monthly_limit_cents`,
		category, req.MonthlyLimitCents, userID,
	).Scan(&b.ID, &b.Category, &b.MonthlyLimitCents)
	if err != nil {
		log.Printf("finances: upsert budget failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}

// @Summary Delete a budget
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param category path string true "Expense category"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 404 {object} httpx.APIError
// @Router /finances/budgets/{category} [delete]
func (h *handler) DeleteBudget(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	category := chi.URLParam(r, "category")
	query := `DELETE FROM budgets WHERE category = $1`
	args := []any{category}
	query, args = scopeToOwner(query, args, role, userID)
	res, err := h.db.Exec(query, args...)
	if err != nil {
		log.Printf("finances: delete budget failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "budget not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}
