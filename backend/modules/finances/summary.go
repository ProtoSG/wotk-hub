package finances

import (
	"log"
	"net/http"
	"workhub/httpx"
	"workhub/middleware"
)

// Summary scopes every aggregate query by created_by for non-admin roles
// (see scopeToOwner), so a guest only ever sees totals over their own
// transactions. Admins see everything unscoped, including legacy pre-auth
// rows where created_by is NULL.
func (h *handler) Summary(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	start, end, err := monthRange(r.URL.Query().Get("month"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var s Summary

	// All-time balance
	balanceQuery, balanceArgs := scopeToOwner(
		`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount_cents ELSE -amount_cents END), 0)
		 FROM transactions WHERE 1=1`, []any{}, role, userID)
	err = h.db.QueryRow(balanceQuery, balanceArgs...).Scan(&s.BalanceCents)
	if err != nil {
		log.Printf("finances: summary balance query failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Selected month income/expense
	monthQuery, monthArgs := scopeToOwner(
		`SELECT
		   COALESCE(SUM(amount_cents) FILTER (WHERE type = 'income'), 0),
		   COALESCE(SUM(amount_cents) FILTER (WHERE type = 'expense'), 0)
		 FROM transactions WHERE occurred_on >= $1 AND occurred_on < $2`,
		[]any{start, end}, role, userID)
	err = h.db.QueryRow(monthQuery, monthArgs...).Scan(&s.MonthIncomeCents, &s.MonthExpenseCents)
	if err != nil {
		log.Printf("finances: summary month query failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// 6-month trend ending at the selected month, zero-filled
	trendStart := start.AddDate(0, -5, 0)
	buckets := map[string]*TrendPoint{}
	s.MonthlyTrend = make([]TrendPoint, 6)
	for i := 0; i < 6; i++ {
		m := trendStart.AddDate(0, i, 0).Format("2006-01")
		s.MonthlyTrend[i] = TrendPoint{Month: m}
		buckets[m] = &s.MonthlyTrend[i]
	}
	trendQuery, trendArgs := scopeToOwner(
		`SELECT to_char(date_trunc('month', occurred_on), 'YYYY-MM') AS m, type, SUM(amount_cents)
		 FROM transactions WHERE occurred_on >= $1 AND occurred_on < $2`,
		[]any{trendStart, end}, role, userID)
	rows, err := h.db.Query(trendQuery+" GROUP BY m, type", trendArgs...)
	if err != nil {
		log.Printf("finances: summary trend query failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var m, typ string
		var sum int64
		if err := rows.Scan(&m, &typ, &sum); err != nil {
			log.Printf("finances: summary trend scan failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if p, ok := buckets[m]; ok {
			if typ == "income" {
				p.IncomeCents = sum
			} else {
				p.ExpenseCents = sum
			}
		}
	}

	// Month expense breakdown by category
	catQuery, catArgs := scopeToOwner(
		`SELECT category, SUM(amount_cents)
		 FROM transactions
		 WHERE type = 'expense' AND occurred_on >= $1 AND occurred_on < $2`,
		[]any{start, end}, role, userID)
	catRows, err := h.db.Query(catQuery+" GROUP BY category ORDER BY 2 DESC", catArgs...)
	if err != nil {
		log.Printf("finances: summary category query failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer catRows.Close()
	s.CategoryBreakdown = []CategoryAmount{}
	for catRows.Next() {
		var c CategoryAmount
		if err := catRows.Scan(&c.Category, &c.AmountCents); err != nil {
			log.Printf("finances: summary category scan failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		s.CategoryBreakdown = append(s.CategoryBreakdown, c)
	}

	httpx.WriteJSON(w, http.StatusOK, s)
}
