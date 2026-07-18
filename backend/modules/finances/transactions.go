package finances

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

// transactionColumns is the select list every transaction read shares, in the
// order scanTransaction expects.
const transactionColumns = `id, type, amount_cents, category, description, occurred_on, created_at, card_id, from_card_id, to_card_id`

func scanTransaction(row interface{ Scan(...any) error }) (Transaction, error) {
	var t Transaction
	var occurredOn, createdAt time.Time
	var cardID, fromCardID, toCardID sql.NullInt64
	err := row.Scan(&t.ID, &t.Type, &t.AmountCents, &t.Category, &t.Description, &occurredOn, &createdAt, &cardID, &fromCardID, &toCardID)
	if err != nil {
		return t, err
	}
	if cardID.Valid {
		t.CardID = &cardID.Int64
	}
	if fromCardID.Valid {
		t.FromCardID = &fromCardID.Int64
	}
	if toCardID.Valid {
		t.ToCardID = &toCardID.Int64
	}
	t.Date = occurredOn.Format(dateLayout)
	t.CreatedAt = createdAt.Format(time.RFC3339)
	return t, nil
}

// cardBalance locks the card row (a pure mutex — balance isn't stored on it
// anymore, see Card in types.go) and returns its live-computed balance and
// used-credit, excluding excludeTxID if given (0 = exclude nothing). Every
// expense deducts from balance; when the card carries a credit limit
// (credit_limit_cents > 0), the same expense also accrues used_credit —
// credit tracking is now inferred purely from credit_limit_cents, not from
// a card type. Excluding the transaction being edited is what makes
// UpdateTransaction's balance check correct: without it, a transaction's
// own prior amount would be double-counted against itself. Returns
// sql.ErrNoRows if the card doesn't exist or is archived.
func cardBalance(tx *sql.Tx, cardID int64, excludeTxID int64) (balanceCents, usedCreditCents int64, err error) {
	var creditLimitCents int64
	err = tx.QueryRow(
		`SELECT credit_limit_cents FROM cards WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		cardID,
	).Scan(&creditLimitCents)
	if err != nil {
		return 0, 0, err
	}

	err = tx.QueryRow(
		`SELECT
		   COALESCE(SUM(CASE
		     WHEN to_card_id = $1 THEN amount_cents
		     WHEN from_card_id = $1 THEN -amount_cents
		     WHEN card_id = $1 AND type = 'income'  THEN amount_cents
		     WHEN card_id = $1 AND type = 'expense' THEN -amount_cents
		     ELSE 0
		   END), 0),
		   COALESCE(SUM(CASE
		     WHEN card_id = $1 AND type = 'expense' AND $2 > 0 THEN amount_cents
		     ELSE 0
		   END), 0)
		 FROM transactions
		 WHERE deleted_at IS NULL AND id != $3
		   AND (to_card_id = $1 OR from_card_id = $1 OR card_id = $1)`,
		cardID, creditLimitCents, excludeTxID,
	).Scan(&balanceCents, &usedCreditCents)
	return balanceCents, usedCreditCents, err
}

// lockTransaction reads the row a write is about to change and holds it
// until commit, so a concurrent edit can't act on stale data.
func lockTransaction(tx *sql.Tx, id int64, role string, userID int64) (old Transaction, err error) {
	query := `SELECT ` + transactionColumns + ` FROM transactions WHERE id = $1 AND deleted_at IS NULL`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)
	query += ` FOR UPDATE`
	return scanTransaction(tx.QueryRow(query, args...))
}

// ListTransactions scopes results by created_by for guests (their personal
// ledger only). Admins see everything, including legacy pre-auth rows where
// created_by is NULL.
// ListTransactions returns transactions, scoped to the caller's own for
// guests, everything for admins. Excludes transfers unless ?type=transfer.
//
// @Summary List transactions
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param month query string false "Filter by month, YYYY-MM"
// @Param type query string false "Filter by type (income, expense, transfer)"
// @Param category query string false "Filter by category"
// @Success 200 {object} listTransactionsResponse
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Router /finances/transactions [get]
func (h *handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	query := `SELECT ` + transactionColumns + `
		FROM transactions WHERE deleted_at IS NULL`
	args := []any{}

	query, args = scopeToOwner(query, args, role, userID)
	if month := q.Get("month"); month != "" {
		start, end, err := monthRange(month)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
			return
		}
		args = append(args, start, end)
		query += " AND occurred_on >= $" + itoa(len(args)-1) + " AND occurred_on < $" + itoa(len(args))
	}
	if t := q.Get("type"); t != "" {
		args = append(args, t)
		query += " AND type = $" + itoa(len(args))
	} else {
		// Movimientos never surfaces transfers (reload, goal contribution,
		// card-to-card) — they're visible through their own originating
		// screen (Tarjetas/Metas), not the general ledger. An explicit
		// ?type=transfer still works, for any future dedicated view.
		query += " AND type != 'transfer'"
	}
	if c := q.Get("category"); c != "" {
		args = append(args, c)
		query += " AND category = $" + itoa(len(args))
	}
	query += " ORDER BY occurred_on DESC, id DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		log.Printf("finances: list transactions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	transactions := []Transaction{}
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			log.Printf("finances: scan transaction failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		transactions = append(transactions, t)
	}
	httpx.WriteJSON(w, http.StatusOK, listTransactionsResponse{Transactions: transactions})
}

// CreateTransaction always stamps created_by from the authenticated user —
// provenance for admin, the ownership boundary guests are scoped by.
// type='transfer' is rejected here: a transfer is only ever created as a
// side effect of CreateCard (seed), CreateContribution, or CreateCardTransfer —
// the reload flow that used to write one was removed by the mandatory-card
// model (see SPEC.md decision log).
// CreateTransaction creates a new income or expense transaction tagged to a
// card, stamping created_by from the authenticated user.
//
// @Summary Create a transaction
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body transactionRequest true "Transaction details"
// @Success 201 {object} Transaction
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError "card not found"
// @Router /finances/transactions [post]
func (h *handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req transactionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	date, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	if err := h.categoryExists(req.Category, req.Type); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid category: "+req.Category)
		return
	} else if err != nil {
		log.Printf("finances: create transaction category check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// cardId is mandatory (see validate); always resolve ownership first so
	// a wrong-owner card surfaces as a clean 404 before the write opens.
	if err := h.cardOwned(req.CardID, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: create transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: create transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	if req.Type == "expense" {
		balanceCents, _, err := cardBalance(tx, req.CardID, 0)
		if err != nil {
			log.Printf("finances: create transaction balance check failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		if balanceCents < req.AmountCents {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente en tarjeta")
			return
		}
	}

	row := tx.QueryRow(
		`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, card_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+transactionColumns,
		req.Type, req.AmountCents, req.Category, req.Description, date, userID, req.CardID,
	)
	t, err := scanTransaction(row)
	if err != nil {
		log.Printf("finances: create transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: create transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

// UpdateTransaction 404s a guest trying to edit a transaction they don't own
// (created_by != their id) rather than revealing it exists, and 404s an
// attempt to edit a transfer row — those are only ever changed from their
// originating flow (Tarjetas/Metas), not from Movimientos.
// UpdateTransaction updates an existing income/expense transaction. Transfer
// rows and transactions owned by another guest are rejected as 404.
//
// @Summary Update a transaction
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Transaction ID"
// @Param body body transactionRequest true "Transaction details"
// @Success 200 {object} Transaction
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /finances/transactions/{id} [put]
func (h *handler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
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
	var req transactionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	date, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	if err := h.categoryExists(req.Category, req.Type); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid category: "+req.Category)
		return
	} else if err != nil {
		log.Printf("finances: update transaction category check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// The new card is resolved before opening the write so an unowned cardId
	// is a clean 404 rather than a rolled-back write. validate already
	// required cardId, so this always runs.
	if err := h.cardOwned(req.CardID, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	old, err := lockTransaction(tx, id, role, userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if old.Type == transactionTypeTransfer {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}

	if req.Type == "expense" {
		balanceCents, _, err := cardBalance(tx, req.CardID, id)
		if err != nil {
			log.Printf("finances: update transaction balance check failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		if balanceCents < req.AmountCents {
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente en tarjeta")
			return
		}
	}

	query := `UPDATE transactions
		 SET type = $1, amount_cents = $2, category = $3, description = $4, occurred_on = $5, card_id = $6
		 WHERE id = $7`
	args := []any{req.Type, req.AmountCents, req.Category, req.Description, date, req.CardID, id}
	query, args = scopeToOwner(query, args, role, userID)
	query += ` RETURNING ` + transactionColumns

	t, err := scanTransaction(tx.QueryRow(query, args...))
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

// DeleteTransaction 404s a guest trying to delete a transaction they don't
// own, and 404s an attempt to delete a transfer row (see UpdateTransaction).
// Soft delete: the row stays for history. Balance reflects the deletion
// automatically the next time it's computed — there's nothing to reverse.
// DeleteTransaction soft-deletes an income/expense transaction. Transfer
// rows and transactions owned by another guest are rejected as 404.
//
// @Summary Delete a transaction
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param id path int true "Transaction ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /finances/transactions/{id} [delete]
func (h *handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
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

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: delete transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	old, err := lockTransaction(tx, id, role, userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("finances: delete transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if old.Type == transactionTypeTransfer {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}

	query := `UPDATE transactions SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)

	res, err := tx.Exec(query, args...)
	if err != nil {
		log.Printf("finances: delete transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: delete transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// RefundTransaction creates a new income transaction that reimburses an expense.
// The original expense row is left unchanged. old.Type != "expense" already
// rejects transfer rows here (a transfer is never "expense").
// RefundTransaction creates a compensating income transaction that
// reimburses an existing expense; the original expense row is unchanged.
//
// @Summary Refund a transaction
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param id path int true "Transaction ID (must be an expense)"
// @Success 201 {object} Transaction
// @Failure 400 {object} httpx.APIError "not an expense"
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /finances/transactions/{id}/refund [post]
func (h *handler) RefundTransaction(w http.ResponseWriter, r *http.Request) {
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

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: refund transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	// Lock and fetch the original transaction.
	old, err := lockTransaction(tx, id, role, userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("finances: refund transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if old.Type != "expense" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "only expense transactions can be refunded")
		return
	}

	// Build the compensating income transaction.
	refundDesc := "Reembolso: " + old.Description
	refundDate := time.Now().Format(dateLayout)

	// The compensating income row is tagged to old.CardID, so cardBalance's
	// income branch repone the card's saldo automatically. Refund of an
	// expense on a card with a credit limit does not reduce
	// used_credit_cents — a known edge deferred to the credit_lines split.
	// See SPEC.md decision log (mandatory-card entry).
	row := tx.QueryRow(
		`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, card_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+transactionColumns,
		"income", old.AmountCents, old.Category, refundDesc, refundDate, userID, old.CardID,
	)
	t, err := scanTransaction(row)
	if err != nil {
		log.Printf("finances: refund transaction insert failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: refund transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}
