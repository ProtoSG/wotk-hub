package finances

import (
	"database/sql"
	"log"
	"net/http"
	"slices"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

// transactionColumns is the select list every transaction read shares, in the
// order scanTransaction expects.
const transactionColumns = `id, type, amount_cents, category, description, occurred_on, created_at, card_id`

func scanTransaction(row interface{ Scan(...any) error }) (Transaction, error) {
	var t Transaction
	var occurredOn, createdAt time.Time
	var cardID sql.NullInt64
	err := row.Scan(&t.ID, &t.Type, &t.AmountCents, &t.Category, &t.Description, &occurredOn, &createdAt, &cardID)
	if err != nil {
		return t, err
	}
	if cardID.Valid {
		t.CardID = &cardID.Int64
	}
	t.Date = occurredOn.Format(dateLayout)
	t.CreatedAt = createdAt.Format(time.RFC3339)
	return t, nil
}

// applyCardDeltas writes accumulated adjustments, ordered by card id so two
// concurrent edits touching the same pair of cards can't deadlock each other.
func applyCardDeltas(tx *sql.Tx, deltas map[int64]cardDelta) error {
	ids := make([]int64, 0, len(deltas))
	for id := range deltas {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		d := deltas[id]
		if d.isZero() {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE cards SET balance_cents = balance_cents + $1, used_credit_cents = used_credit_cents + $2
			 WHERE id = $3`,
			d.balanceCents, d.usedCreditCents, id,
		); err != nil {
			return err
		}
	}
	return nil
}

func addDelta(deltas map[int64]cardDelta, cardID int64, d cardDelta) {
	acc := deltas[cardID]
	acc.balanceCents += d.balanceCents
	acc.usedCreditCents += d.usedCreditCents
	deltas[cardID] = acc
}

// cardTypeOwned resolves the type of a card the caller is tagging a
// transaction to. It is scoped: tagging a card you don't own must not work,
// and must not reveal that the card exists.
func (h *handler) cardTypeOwned(cardID int64, role string, userID int64) (string, error) {
	query := `SELECT type FROM cards WHERE id = $1`
	args := []any{cardID}
	query, args = scopeToOwner(query, args, role, userID)
	var t string
	err := h.db.QueryRow(query, args...).Scan(&t)
	return t, err
}

// cardTypeByID resolves the type of the card a transaction is already tagged
// to, so its adjustment can be reversed. Unscoped on purpose: ownership was
// settled when the tag was written, and the type is only used to compute a
// delta, never returned to the caller.
func cardTypeByID(tx *sql.Tx, cardID int64) (string, error) {
	var t string
	err := tx.QueryRow(`SELECT type FROM cards WHERE id = $1`, cardID).Scan(&t)
	return t, err
}

// lockTransaction reads the row a write is about to change and holds it until
// commit, so a concurrent edit can't compute its card delta from stale
// amounts. Returns the delta the stored row currently contributes to its card.
func lockTransaction(tx *sql.Tx, id int64, role string, userID int64) (old Transaction, oldDelta cardDelta, err error) {
	query := `SELECT ` + transactionColumns + ` FROM transactions WHERE id = $1`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)
	query += ` FOR UPDATE`

	old, err = scanTransaction(tx.QueryRow(query, args...))
	if err != nil {
		return old, cardDelta{}, err
	}
	if old.CardID == nil {
		return old, cardDelta{}, nil
	}
	cardType, err := cardTypeByID(tx, *old.CardID)
	if err == sql.ErrNoRows {
		// The card is gone but the tag survived; there is no counter left to
		// restore, so let the write proceed rather than stranding the row.
		log.Printf("finances: transaction %d tagged to missing card %d, skipping reversal", id, *old.CardID)
		return old, cardDelta{}, nil
	}
	if err != nil {
		return old, cardDelta{}, err
	}
	return old, cardAdjustment(cardType, old.Type, old.AmountCents), nil
}

// ListTransactions scopes results by created_by for guests (their personal
// ledger only). Admins see everything, including legacy pre-auth rows where
// created_by is NULL.
func (h *handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	query := `SELECT ` + transactionColumns + `
		FROM transactions WHERE 1=1`
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"transactions": transactions})
}

// CreateTransaction always stamps created_by from the authenticated user —
// provenance for admin, the ownership boundary guests are scoped by. When the
// transaction is tagged to a card, the insert and the card's counters move
// together or not at all.
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

	var delta cardDelta
	if req.CardID != nil {
		cardType, err := h.cardTypeOwned(*req.CardID, role, userID)
		if err == sql.ErrNoRows {
			httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
			return
		} else if err != nil {
			log.Printf("finances: create transaction failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		delta = cardAdjustment(cardType, req.Type, req.AmountCents)
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: create transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

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

	if req.CardID != nil && !delta.isZero() && delta.balanceCents < 0 {
		var balanceCents int64
		if err := tx.QueryRow(`SELECT balance_cents FROM cards WHERE id = $1 FOR UPDATE`, *req.CardID).Scan(&balanceCents); err != nil {
			tx.Rollback()
			log.Printf("finances: create transaction failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		if balanceCents < -delta.balanceCents {
			tx.Rollback()
			httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente en tarjeta")
			return
		}
	}

	if err := applyCardDeltas(tx, map[int64]cardDelta{*req.CardID: delta}); err != nil {
		log.Printf("finances: create transaction card adjustment failed: %v", err)
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
// (created_by != their id) rather than revealing it exists.
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

	// The new card is resolved before opening the write so an unowned cardId
	// is a clean 404 rather than a rolled-back write.
	var newDelta cardDelta
	if req.CardID != nil {
		cardType, err := h.cardTypeOwned(*req.CardID, role, userID)
		if err == sql.ErrNoRows {
			httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
			return
		} else if err != nil {
			log.Printf("finances: update transaction failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		newDelta = cardAdjustment(cardType, req.Type, req.AmountCents)
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	old, oldDelta, err := lockTransaction(tx, id, role, userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("finances: update transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Undo what the stored row contributed, then apply what the new one does.
	// When the card is unchanged both land on the same key and collapse into a
	// single net UPDATE.
	deltas := map[int64]cardDelta{}
	if old.CardID != nil {
		addDelta(deltas, *old.CardID, oldDelta.reverse())
	}
	if req.CardID != nil {
		addDelta(deltas, *req.CardID, newDelta)
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

	if err := applyCardDeltas(tx, deltas); err != nil {
		log.Printf("finances: update transaction card adjustment failed: %v", err)
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
// own. Deleting a tagged transaction gives the card back what the transaction
// took from it.
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

	old, oldDelta, err := lockTransaction(tx, id, role, userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("finances: delete transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	query := `DELETE FROM transactions WHERE id = $1`
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

	if old.CardID != nil {
		if err := applyCardDeltas(tx, map[int64]cardDelta{*old.CardID: oldDelta.reverse()}); err != nil {
			log.Printf("finances: delete transaction card adjustment failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: delete transaction failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

// RefundTransaction creates a new income transaction that reimburses an expense.
// The original expense row is left unchanged.
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
	old, _, err := lockTransaction(tx, id, role, userID)
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

	// Note: incomes with a cardId do not auto-update the card balance
	// (cardAdjustment returns zero delta for income type). This is the
	// documented limitation — the refund restores the ledger balance
	// but does not reload the card.
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
