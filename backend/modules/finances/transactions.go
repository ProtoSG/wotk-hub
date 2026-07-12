package finances

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

func scanTransaction(row interface{ Scan(...any) error }) (Transaction, error) {
	var t Transaction
	var occurredOn, createdAt time.Time
	err := row.Scan(&t.ID, &t.Type, &t.AmountCents, &t.Category, &t.Description, &occurredOn, &createdAt)
	if err != nil {
		return t, err
	}
	t.Date = occurredOn.Format(dateLayout)
	t.CreatedAt = createdAt.Format(time.RFC3339)
	return t, nil
}

// ListTransactions scopes results by created_by for guests (their personal
// ledger only). Admins see everything, including legacy pre-auth rows where
// created_by is NULL.
func (h *handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	query := `SELECT id, type, amount_cents, category, description, occurred_on, created_at
		FROM transactions WHERE 1=1`
	args := []any{}

	query, args = scopeToOwner(query, args, role, userID)
	if month := q.Get("month"); month != "" {
		start, end, err := monthRange(month)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		args = append(args, start, end)
		query += " AND occurred_on >= $" + itoa(len(args)-1) + " AND occurred_on < $" + itoa(len(args))
	}
	if t := q.Get("type"); t != "" {
		args = append(args, t)
		query += " AND type = $" + itoa(len(args))
	}
	if c := q.Get("category"); c != "" {
		args = append(args, c)
		query += " AND category = $" + itoa(len(args))
	}
	query += " ORDER BY occurred_on DESC, id DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		log.Printf("finances: list transactions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	transactions := []Transaction{}
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			log.Printf("finances: scan transaction failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		transactions = append(transactions, t)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"transactions": transactions})
}

// CreateTransaction always stamps created_by from the authenticated user —
// provenance for admin, the ownership boundary guests are scoped by.
func (h *handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req transactionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	row := h.db.QueryRow(
		`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, type, amount_cents, category, description, occurred_on, created_at`,
		req.Type, req.AmountCents, req.Category, req.Description, date, userID,
	)
	t, err := scanTransaction(row)
	if err != nil {
		log.Printf("finances: create transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

// UpdateTransaction 404s a guest trying to edit a transaction they don't own
// (created_by != their id) rather than revealing it exists.
func (h *handler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req transactionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	query := `UPDATE transactions
		 SET type = $1, amount_cents = $2, category = $3, description = $4, occurred_on = $5
		 WHERE id = $6`
	args := []any{req.Type, req.AmountCents, req.Category, req.Description, date, id}
	query, args = scopeToOwner(query, args, role, userID)
	query += ` RETURNING id, type, amount_cents, category, description, occurred_on, created_at`

	row := h.db.QueryRow(query, args...)
	t, err := scanTransaction(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

// DeleteTransaction 404s a guest trying to delete a transaction they don't own.
func (h *handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	query := `DELETE FROM transactions WHERE id = $1`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)

	res, err := h.db.Exec(query, args...)
	if err != nil {
		log.Printf("finances: delete transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, "transaction not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
