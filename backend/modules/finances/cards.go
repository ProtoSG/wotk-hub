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

// cardsBaseQuery is the read shape every card lookup shares — balance and
// used-credit are computed live from transactions (see cardBalance in
// transactions.go for the lock-and-check variant used by writes; this is
// the read-only equivalent for lists and single lookups). Column order
// matches scanCard.
const cardsBaseQuery = `
	SELECT
	  c.id, c.name, c.type, c.bank, c.last4, c.color, c.icon,
	  c.credit_limit_cents, c.created_at,
	  COALESCE((SELECT SUM(CASE
	    WHEN t.to_card_id = c.id THEN t.amount_cents
	    WHEN t.from_card_id = c.id THEN -t.amount_cents
	    WHEN t.card_id = c.id AND t.type = 'income'  AND c.type != 'credito' THEN t.amount_cents
	    WHEN t.card_id = c.id AND t.type = 'expense' AND c.type != 'credito' THEN -t.amount_cents
	    ELSE 0 END)
	   FROM transactions t
	   WHERE t.deleted_at IS NULL
	     AND (t.to_card_id = c.id OR t.from_card_id = c.id OR t.card_id = c.id)), 0) AS balance_cents,
	  COALESCE((SELECT SUM(t.amount_cents) FROM transactions t
	   WHERE t.deleted_at IS NULL AND t.card_id = c.id AND t.type = 'expense' AND c.type = 'credito'), 0) AS used_credit_cents
	FROM cards c
	WHERE c.deleted_at IS NULL`

func scanCard(row interface{ Scan(...any) error }) (Card, error) {
	var c Card
	var createdAt time.Time
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.Bank, &c.Last4, &c.Color, &c.Icon,
		&c.CreditLimitCents, &createdAt, &c.BalanceCents, &c.UsedCreditCents)
	if err != nil {
		return c, err
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	return c, nil
}

// getCard fetches one card's computed view, unscoped by owner — callers
// that need ownership enforced check it separately (cardOwned/
// cardTypeOwned) before calling this.
func (h *handler) getCard(id int64) (Card, error) {
	row := h.db.QueryRow(cardsBaseQuery+" AND c.id = $1", id)
	return scanCard(row)
}

func scanCardReload(row interface{ Scan(...any) error }) (CardReload, error) {
	var cr CardReload
	var occurredOn, createdAt time.Time
	err := row.Scan(&cr.ID, &cr.CardID, &cr.AmountCents, &occurredOn, &cr.Note, &createdAt)
	if err != nil {
		return cr, err
	}
	cr.Date = occurredOn.Format(dateLayout)
	cr.CreatedAt = createdAt.Format(time.RFC3339)
	return cr, nil
}

// ListCards scopes results by created_by for guests (their personal cards
// only). Admins see everything, including legacy pre-auth rows where
// created_by is NULL.
func (h *handler) ListCards(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	query := cardsBaseQuery
	args := []any{}
	query, args = scopeToOwner(query, args, role, userID)
	query += " ORDER BY c.id DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		log.Printf("finances: list cards failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	cards := []Card{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			log.Printf("finances: scan card failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		cards = append(cards, c)
	}
	httpx.WriteJSON(w, http.StatusOK, listCardsResponse{Cards: cards})
}

// CreateCard always stamps created_by from the authenticated user. A
// starting balance becomes a seed transfer (type='transfer', to_card_id
// set) in the same DB transaction as the card insert, instead of a stored
// initial_balance_cents column — see SPEC.md.
func (h *handler) CreateCard(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req cardRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var initial, creditLimit int64
	if req.InitialBalanceCents != nil {
		initial = *req.InitialBalanceCents
	}
	if req.CreditLimitCents != nil {
		creditLimit = *req.CreditLimitCents
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: create card failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	var c Card
	var createdAt time.Time
	err = tx.QueryRow(
		`INSERT INTO cards (name, type, bank, last4, color, icon, credit_limit_cents, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, name, type, bank, last4, color, icon, credit_limit_cents, created_at`,
		req.Name, req.Type, req.Bank, req.Last4, req.Color, req.Icon, creditLimit, userID,
	).Scan(&c.ID, &c.Name, &c.Type, &c.Bank, &c.Last4, &c.Color, &c.Icon, &c.CreditLimitCents, &createdAt)
	if err != nil {
		log.Printf("finances: create card failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)

	if initial > 0 {
		if _, err := tx.Exec(
			`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, to_card_id)
			 VALUES ('transfer', $1, $2, 'Saldo inicial', CURRENT_DATE, $3, $4)`,
			initial, transferCategory, userID, c.ID,
		); err != nil {
			log.Printf("finances: create card seed transfer failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		c.BalanceCents = initial
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: create card failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

// UpdateCard 404s a guest trying to edit a card they don't own
// (created_by != their id) rather than revealing it exists. Balance only
// changes via a reload, a transfer, or a tagged transaction, never through
// this endpoint.
func (h *handler) UpdateCard(w http.ResponseWriter, r *http.Request) {
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
	var req cardRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	// req.CreditLimitCents is a pointer so an omitted field means "keep
	// what's stored" — same convention as the rest of cardRequest's
	// pointer fields (see its doc comment).
	query := `UPDATE cards
		 SET name = $1, type = $2, bank = $3, last4 = $4, color = $5, icon = $6,
		     credit_limit_cents = COALESCE($7, credit_limit_cents)
		 WHERE id = $8 AND deleted_at IS NULL`
	args := []any{req.Name, req.Type, req.Bank, req.Last4, req.Color, req.Icon, req.CreditLimitCents, id}
	query, args = scopeToOwner(query, args, role, userID)
	query += ` RETURNING id`

	var updatedID int64
	err = h.db.QueryRow(query, args...).Scan(&updatedID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	}
	if err != nil {
		log.Printf("finances: update card failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	c, err := h.getCard(updatedID)
	if err != nil {
		log.Printf("finances: update card refetch failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c)
}

// DeleteCard 404s a guest trying to delete a card they don't own. Soft
// delete: a card's reloads and transactions are financial history worth
// keeping, and other rows (transactions.card_id/from_card_id/to_card_id,
// savings_goals.default_card_id) keep referencing it, so the row is
// archived (deleted_at set) instead of removed.
func (h *handler) DeleteCard(w http.ResponseWriter, r *http.Request) {
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

	query := `UPDATE cards SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)

	res, err := h.db.Exec(query, args...)
	if err != nil {
		log.Printf("finances: delete card failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// cardOwned checks that the card exists, isn't archived, and is owned by the
// caller (or the caller is admin), 404ing otherwise without revealing
// existence.
func (h *handler) cardOwned(id int64, role string, userID int64) error {
	query := `SELECT id FROM cards WHERE id = $1 AND deleted_at IS NULL`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)
	var got int64
	return h.db.QueryRow(query, args...).Scan(&got)
}

// ListReloads 404s if the card isn't owned by the caller. Reads reload
// history from transactions instead of a separate card_reloads table — a
// reload is a transfer with no source card (from_card_id IS NULL).
func (h *handler) ListReloads(w http.ResponseWriter, r *http.Request) {
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
	if err := h.cardOwned(id, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: list reloads failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	rows, err := h.db.Query(
		`SELECT id, to_card_id, amount_cents, occurred_on, description, created_at
		 FROM transactions
		 WHERE deleted_at IS NULL AND type = 'transfer' AND from_card_id IS NULL AND to_card_id = $1
		 ORDER BY occurred_on DESC, id DESC`,
		id,
	)
	if err != nil {
		log.Printf("finances: list reloads failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	reloads := []CardReload{}
	for rows.Next() {
		cr, err := scanCardReload(rows)
		if err != nil {
			log.Printf("finances: scan reload failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		reloads = append(reloads, cr)
	}
	httpx.WriteJSON(w, http.StatusOK, listReloadsResponse{Reloads: reloads})
}

// CreateReload 404s if the card isn't owned by the caller, then inserts a
// transfer transaction (from_card_id NULL, to_card_id = this card) instead
// of a card_reloads row + direct balance UPDATE — the card's balance
// reflects it the next time it's computed, nothing to keep in sync.
func (h *handler) CreateReload(w http.ResponseWriter, r *http.Request) {
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
	if err := h.cardOwned(id, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: create reload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var req cardReloadRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	date, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	description := req.Note
	if description == "" {
		description = "Recarga"
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: create reload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	row := tx.QueryRow(
		`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, to_card_id)
		 VALUES ('transfer', $1, $2, $3, $4, $5, $6)
		 RETURNING id, to_card_id, amount_cents, occurred_on, description, created_at`,
		req.AmountCents, transferCategory, description, date, userID, id,
	)
	cr, err := scanCardReload(row)
	if err != nil {
		log.Printf("finances: create reload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: create reload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, cr)
}

// CreateCardTransfer moves money between two of the caller's own cards in
// one transaction row (from_card_id and to_card_id both set). Credit cards
// are excluded on either side — v1 restriction, see SPEC.md.
func (h *handler) CreateCardTransfer(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}
	var req cardTransferRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	date, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}

	fromType, err := h.cardTypeOwned(req.FromCardID, role, userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: create card transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	toType, err := h.cardTypeOwned(req.ToCardID, role, userID)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: create card transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if fromType == cardTypeCredit || toType == cardTypeCredit {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "solo se puede transferir entre débito/prepago")
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: create card transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	// Lock both cards in id order so a concurrent transfer in the opposite
	// direction can't deadlock against this one.
	ids := []int64{req.FromCardID, req.ToCardID}
	slices.Sort(ids)
	for _, cid := range ids {
		if _, err := tx.Exec(`SELECT id FROM cards WHERE id = $1 FOR UPDATE`, cid); err != nil {
			log.Printf("finances: create card transfer lock failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
	}

	balanceCents, _, _, err := cardBalance(tx, req.FromCardID, 0)
	if err != nil {
		log.Printf("finances: create card transfer balance check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if balanceCents < req.AmountCents {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente")
		return
	}

	description := req.Note
	if description == "" {
		description = "Transferencia entre tarjetas"
	}

	row := tx.QueryRow(
		`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, from_card_id, to_card_id)
		 VALUES ('transfer', $1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+transactionColumns,
		req.AmountCents, transferCategory, description, date, userID, req.FromCardID, req.ToCardID,
	)
	t, err := scanTransaction(row)
	if err != nil {
		log.Printf("finances: create card transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: create card transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}
