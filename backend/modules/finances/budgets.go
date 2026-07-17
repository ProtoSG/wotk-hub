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
	query, args := scopeToOwner(
		`SELECT b.id, b.category, b.monthly_limit_cents, COALESCE(SUM(t.amount_cents), 0) AS spent
		 FROM budgets b
		 LEFT JOIN transactions t
		   ON t.category = b.category AND t.type = 'expense' AND t.deleted_at IS NULL
		  AND t.occurred_on >= $1 AND t.occurred_on < $2`,
		[]any{start, end}, role, userID)
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

func (h *handler) UpsertBudget(w http.ResponseWriter, r *http.Request) {
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
	err := h.db.QueryRow(
		`INSERT INTO budgets (category, monthly_limit_cents)
		 VALUES ($1, $2)
		 ON CONFLICT (category) DO UPDATE SET monthly_limit_cents = EXCLUDED.monthly_limit_cents
		 RETURNING id, category, monthly_limit_cents`,
		category, req.MonthlyLimitCents,
	).Scan(&b.ID, &b.Category, &b.MonthlyLimitCents)
	if err != nil {
		log.Printf("finances: upsert budget failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, b)
}

func (h *handler) DeleteBudget(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "category")
	res, err := h.db.Exec(`DELETE FROM budgets WHERE category = $1`, category)
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
