package finances

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

// cardColumns is the select list every card read shares, in the order
// scanCard expects.
const cardColumns = `id, name, type, bank, last4, color, icon, balance_cents,
	initial_balance_cents, credit_limit_cents, used_credit_cents, created_at`

func scanCard(row interface{ Scan(...any) error }) (Card, error) {
	var c Card
	var createdAt time.Time
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.Bank, &c.Last4, &c.Color, &c.Icon, &c.BalanceCents,
		&c.InitialBalanceCents, &c.CreditLimitCents, &c.UsedCreditCents, &createdAt)
	if err != nil {
		return c, err
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	return c, nil
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

	query := `SELECT ` + cardColumns + `
		FROM cards WHERE 1=1`
	args := []any{}
	query, args = scopeToOwner(query, args, role, userID)
	query += " ORDER BY id DESC"

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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

// CreateCard always stamps created_by from the authenticated user. A card
// opens at its initial balance; from there only reloads and tagged
// transactions move it.
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
	row := h.db.QueryRow(
		`INSERT INTO cards (name, type, bank, last4, color, icon, balance_cents,
		     initial_balance_cents, credit_limit_cents, used_credit_cents, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, 0, $9)
		 RETURNING `+cardColumns,
		req.Name, req.Type, req.Bank, req.Last4, req.Color, req.Icon, initial, creditLimit, userID,
	)
	c, err := scanCard(row)
	if err != nil {
		log.Printf("finances: create card failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

// UpdateCard 404s a guest trying to edit a card they don't own
// (created_by != their id) rather than revealing it exists. Balance only
// changes via a reload, never through this endpoint.
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

	query := `UPDATE cards
		 SET name = $1, type = $2, bank = $3, last4 = $4, color = $5, icon = $6
		 WHERE id = $7`
	args := []any{req.Name, req.Type, req.Bank, req.Last4, req.Color, req.Icon, id}
	query, args = scopeToOwner(query, args, role, userID)
	query += ` RETURNING id, name, type, bank, last4, color, icon, balance_cents, created_at`

	row := h.db.QueryRow(query, args...)
	c, err := scanCard(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	}
	if err != nil {
		log.Printf("finances: update card failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c)
}

// DeleteCard 404s a guest trying to delete a card they don't own.
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

	query := `DELETE FROM cards WHERE id = $1`
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

// cardOwned checks that the card exists and is owned by the caller
// (or the caller is admin), 404ing otherwise without revealing existence.
func (h *handler) cardOwned(id int64, role string, userID int64) error {
	query := `SELECT id FROM cards WHERE id = $1`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)
	var got int64
	return h.db.QueryRow(query, args...).Scan(&got)
}

// ListReloads 404s if the card isn't owned by the caller.
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
		`SELECT id, card_id, amount_cents, occurred_on, note, created_at
		 FROM card_reloads WHERE card_id = $1 ORDER BY occurred_on DESC, id DESC`,
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reloads": reloads})
}

// CreateReload 404s if the card isn't owned by the caller, then atomically
// inserts the reload and increments the card's balance in one transaction.
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

	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("finances: create reload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	row := tx.QueryRow(
		`INSERT INTO card_reloads (card_id, amount_cents, occurred_on, note, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, card_id, amount_cents, occurred_on, note, created_at`,
		id, req.AmountCents, date, req.Note, userID,
	)
	cr, err := scanCardReload(row)
	if err != nil {
		tx.Rollback()
		log.Printf("finances: create reload failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if _, err := tx.Exec(`UPDATE cards SET balance_cents = balance_cents + $1 WHERE id = $2`, req.AmountCents, id); err != nil {
		tx.Rollback()
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
