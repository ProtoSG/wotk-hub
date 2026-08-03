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
	  c.id, c.name, c.bank, c.last4, c.color, c.icon,
	  c.credit_limit_cents, c.created_at,
	  COALESCE((SELECT SUM(CASE
	    WHEN t.to_card_id = c.id THEN t.amount_cents
	    WHEN t.from_card_id = c.id THEN -t.amount_cents
	    WHEN t.card_id = c.id AND t.type = 'income'  THEN t.amount_cents
	    WHEN t.card_id = c.id AND t.type = 'expense' THEN -t.amount_cents
	    ELSE 0 END)
	   FROM transactions t
	   WHERE t.deleted_at IS NULL
	     AND (t.to_card_id = c.id OR t.from_card_id = c.id OR t.card_id = c.id)), 0) AS balance_cents,
	  COALESCE((SELECT SUM(t.amount_cents) FROM transactions t
	   WHERE t.deleted_at IS NULL AND t.card_id = c.id AND t.type = 'expense' AND c.credit_limit_cents > 0), 0) AS used_credit_cents
	FROM cards c
	WHERE c.deleted_at IS NULL`

func scanCard(row interface{ Scan(...any) error }) (Card, error) {
	var c Card
	var createdAt time.Time
	err := row.Scan(&c.ID, &c.Name, &c.Bank, &c.Last4, &c.Color, &c.Icon,
		&c.CreditLimitCents, &createdAt, &c.BalanceCents, &c.UsedCreditCents)
	if err != nil {
		return c, err
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	return c, nil
}

// getCard fetches one card's computed view, unscoped by owner — callers
// that need ownership enforced check it separately (cardOwned) before
// calling this.
func (h *handler) getCard(id int64) (Card, error) {
	row := h.db.QueryRow(cardsBaseQuery+" AND c.id = $1", id)
	return scanCard(row)
}

// ListCards scopes results by created_by for guests (their personal cards
// only). Admins see everything, including legacy pre-auth rows where
// created_by is NULL.
// ListCards returns cards, scoped to the caller's own for guests, everything
// for admins. Balance and used credit are computed live from transactions.
//
// @Summary List cards
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Success 200 {object} listCardsResponse
// @Failure 401 {object} httpx.APIError
// @Router /finances/cards [get]
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
// CreateCard creates a new card. An initialBalanceCents > 0 becomes a seed
// transfer transaction rather than a stored balance column.
//
// @Summary Create a card
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body cardRequest true "Card details"
// @Success 201 {object} Card
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Router /finances/cards [post]
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
		`INSERT INTO cards (name, bank, last4, color, icon, credit_limit_cents, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, name, bank, last4, color, icon, credit_limit_cents, created_at`,
		req.Name, req.Bank, req.Last4, req.Color, req.Icon, creditLimit, userID,
	).Scan(&c.ID, &c.Name, &c.Bank, &c.Last4, &c.Color, &c.Icon, &c.CreditLimitCents, &createdAt)
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
// UpdateCard updates card details. Balance never changes through this
// endpoint — only via a reload, a transfer, or a tagged transaction.
//
// @Summary Update a card
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Card ID"
// @Param body body cardRequest true "Card details"
// @Success 200 {object} Card
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /finances/cards/{id} [put]
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
		 SET name = $1, bank = $2, last4 = $3, color = $4, icon = $5,
		     credit_limit_cents = COALESCE($6, credit_limit_cents)
		 WHERE id = $7 AND deleted_at IS NULL`
	args := []any{req.Name, req.Bank, req.Last4, req.Color, req.Icon, req.CreditLimitCents, id}
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
// delete: a card's transactions are financial history worth keeping, and
// other rows (transactions.card_id/from_card_id/to_card_id,
// savings_goals.default_card_id) keep referencing it, so the row is
// archived (deleted_at set) instead of removed.
//
// Last-active-card invariant (added from scratch — explore R4 confirmed no
// prior first/last-card rule of any kind): archiving the owner's only
// remaining active card would leave the finances module without any card to
// tag against, so it's rejected with 409 before the soft-delete runs. The
// count is scoped the same way the soft-delete is (created_by for guests,
// unscoped for admins seeing legacy NULL rows), so the admin and the
// per-user view agree on "your last card".
// DeleteCard soft-deletes (archives) a card. Rejected with 409 if it's the
// owner's last remaining active card.
//
// @Summary Delete a card
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param id path int true "Card ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 400 {object} httpx.APIError
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Failure 409 {object} httpx.APIError "last active card"
// @Router /finances/cards/{id} [delete]
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

	// Count this owner's active cards BEFORE archiving. If the target is
	// their last one, refuse — the soft-delete below would otherwise leave
	// the owner with no taggable card. Scoped exactly like the UPDATE so
	// the guard and the mutation agree on what "yours" means.
	countQuery := `SELECT COUNT(*) FROM cards WHERE deleted_at IS NULL`
	countArgs := []any{}
	countQuery, countArgs = scopeToOwner(countQuery, countArgs, role, userID)
	var activeCount int64
	if err := h.db.QueryRow(countQuery, countArgs...).Scan(&activeCount); err != nil {
		log.Printf("finances: delete card count failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if activeCount <= 1 {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict, "no podés archivar tu última tarjeta activa")
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

// CreateCardTransfer moves money between two of the caller's own cards in
// one transaction row (from_card_id and to_card_id both set).
// CreateCardTransfer moves money between two of the caller's own cards as a
// single transfer transaction row.
//
// @Summary Transfer between cards
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body cardTransferRequest true "Transfer details"
// @Success 201 {object} Transaction
// @Failure 400 {object} httpx.APIError "insufficient balance or invalid request"
// @Failure 401 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError "card not found"
// @Router /finances/cards/transfers [post]
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

	if err := h.cardOwned(req.FromCardID, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: create card transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if err := h.cardOwned(req.ToCardID, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: create card transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
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

	balanceCents, _, err := cardBalance(tx, req.FromCardID, 0)
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
