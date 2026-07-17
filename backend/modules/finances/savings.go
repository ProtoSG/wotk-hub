package finances

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"
	"workhub/httpx"
	"workhub/middleware"
)

func scanGoal(row interface{ Scan(...any) error }) (SavingsGoal, error) {
	var g SavingsGoal
	var createdAt time.Time
	var deadline sql.NullTime
	err := row.Scan(&g.ID, &g.Name, &g.TargetCents, &g.CurrentCents, &deadline, &g.Icon, &g.Color, &g.DefaultCardID, &g.CreatedBy, &createdAt)
	if err != nil {
		return g, err
	}
	if deadline.Valid {
		g.Deadline = deadline.Time.Format(dateLayout)
	}
	g.CreatedAt = createdAt.Format(time.RFC3339)
	return g, nil
}

func scanContribution(row interface{ Scan(...any) error }) (SavingsContribution, error) {
	var c SavingsContribution
	var createdAt time.Time
	var occurredOn time.Time
	var transactionID sql.NullInt64
	err := row.Scan(&c.ID, &c.GoalID, &c.AmountCents, &occurredOn, &c.Note, &transactionID, &c.CreatedBy, &createdAt)
	if err != nil {
		return c, err
	}
	if transactionID.Valid {
		c.TransactionID = &transactionID.Int64
	}
	c.Date = occurredOn.Format(dateLayout)
	c.CreatedAt = createdAt.Format(time.RFC3339)
	return c, nil
}

// ListGoals returns all savings goals owned by the caller.
func (h *handler) ListGoals(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	query := `SELECT id, name, target_cents, current_cents, deadline, icon, color, default_card_id, created_by, created_at
		FROM savings_goals WHERE deleted_at IS NULL`
	args := []any{}
	query, args = scopeToOwner(query, args, role, userID)
	query += " ORDER BY id DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		log.Printf("finances: list goals failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	goals := []SavingsGoal{}
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			log.Printf("finances: scan goal failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		goals = append(goals, g)
	}
	httpx.WriteJSON(w, http.StatusOK, listGoalsResponse{Goals: goals})
}

// defaultCardOwned checks that the goal's default card is owned by the
// caller and isn't a credito card — a savings goal always draws from a
// real spendable balance, never a credit line. Returns a user-facing error
// string when invalid, nil when ok.
func (h *handler) defaultCardOwned(cardID int64, role string, userID int64) error {
	cardType, err := h.cardTypeOwned(cardID, role, userID)
	if err != nil {
		return err
	}
	if cardType == cardTypeCredit {
		return errCreditDefaultCard
	}
	return nil
}

var errCreditDefaultCard = errors.New("la tarjeta predeterminada no puede ser de crédito")

// CreateGoal creates a new savings goal for the authenticated user.
func (h *handler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	userID, role, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "unauthorized")
		return
	}

	var req savingsGoalRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	if err := h.defaultCardOwned(*req.DefaultCardID, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err == errCreditDefaultCard {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	} else if err != nil {
		log.Printf("finances: create goal card check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	row := h.db.QueryRow(
		`INSERT INTO savings_goals (name, target_cents, current_cents, deadline, icon, color, default_card_id, created_by)
		 VALUES ($1, $2, 0, $3, $4, $5, $6, $7)
		 RETURNING id, name, target_cents, current_cents, deadline, icon, color, default_card_id, created_by, created_at`,
		req.Name, req.TargetCents, req.normalizedDeadline(), req.Icon, req.Color, req.DefaultCardID, userID,
	)
	g, err := scanGoal(row)
	if err != nil {
		log.Printf("finances: create goal failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, g)
}

// UpdateGoal updates an existing savings goal.
func (h *handler) UpdateGoal(w http.ResponseWriter, r *http.Request) {
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

	var req savingsGoalRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	if err := h.defaultCardOwned(*req.DefaultCardID, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err == errCreditDefaultCard {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	} else if err != nil {
		log.Printf("finances: update goal card check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	query := `UPDATE savings_goals
		SET name = $1, target_cents = $2, deadline = $3, icon = $4, color = $5, default_card_id = $6
		WHERE id = $7 AND deleted_at IS NULL`
	args := []any{req.Name, req.TargetCents, req.normalizedDeadline(), req.Icon, req.Color, req.DefaultCardID, id}
	query, args = scopeToOwner(query, args, role, userID)
	query += ` RETURNING id, name, target_cents, current_cents, deadline, icon, color, default_card_id, created_by, created_at`

	row := h.db.QueryRow(query, args...)
	g, err := scanGoal(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
		return
	}
	if err != nil {
		log.Printf("finances: update goal failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, g)
}

// DeleteGoal soft-deletes a savings goal — its contributions and the
// transfer transactions they generated stay in place for history, and
// default_card_id references keep resolving.
func (h *handler) DeleteGoal(w http.ResponseWriter, r *http.Request) {
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

	query := `UPDATE savings_goals SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)

	res, err := h.db.Exec(query, args...)
	if err != nil {
		log.Printf("finances: delete goal failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}

// goalOwned checks that the goal exists, isn't archived, and is owned by the
// caller.
func (h *handler) goalOwned(id int64, role string, userID int64) error {
	query := `SELECT id FROM savings_goals WHERE id = $1 AND deleted_at IS NULL`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)
	var got int64
	return h.db.QueryRow(query, args...).Scan(&got)
}

// ListContributions returns all contributions for a goal.
func (h *handler) ListContributions(w http.ResponseWriter, r *http.Request) {
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
	if err := h.goalOwned(id, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
		return
	} else if err != nil {
		log.Printf("finances: list contributions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	rows, err := h.db.Query(
		`SELECT id, goal_id, amount_cents, occurred_on, note, transaction_id, created_by, created_at
		 FROM savings_contributions WHERE goal_id = $1 ORDER BY occurred_on DESC, id DESC`,
		id,
	)
	if err != nil {
		log.Printf("finances: list contributions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	contributions := []SavingsContribution{}
	for rows.Next() {
		c, err := scanContribution(rows)
		if err != nil {
			log.Printf("finances: scan contribution failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		contributions = append(contributions, c)
	}
	httpx.WriteJSON(w, http.StatusOK, listContributionsResponse{Contributions: contributions})
}

// CreateContribution atomically: inserts the contribution, increments the
// goal's current_cents, and inserts a transfer transaction (from_card_id =
// the goal's default card) that the contribution links back to via
// transaction_id. The default card is now mandatory on every goal — see
// SPEC.md's "Scenarios — goal contribution" for the full case-by-case
// walkthrough (archived default card, insufficient balance, concurrency).
func (h *handler) CreateContribution(w http.ResponseWriter, r *http.Request) {
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

	var req savingsContributionRequest
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
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer tx.Rollback()

	// Lock the goal row so a concurrent UpdateGoal changing default_card_id
	// can't interleave with this contribution.
	var defaultCardID int64
	var goalName string
	query := `SELECT default_card_id, name FROM savings_goals WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`
	args := []any{id}
	query, args = scopeToOwner(query, args, role, userID)
	if err := tx.QueryRow(query, args...).Scan(&defaultCardID, &goalName); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
		return
	} else if err != nil {
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	balanceCents, _, _, err := cardBalance(tx, defaultCardID, 0)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeConflict,
			"la tarjeta predeterminada de esta meta fue eliminada — asigná una nueva antes de aportar")
		return
	}
	if err != nil {
		log.Printf("finances: create contribution balance check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if balanceCents < req.AmountCents {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "saldo insuficiente en tarjeta")
		return
	}

	row := tx.QueryRow(
		`INSERT INTO savings_contributions (goal_id, amount_cents, occurred_on, note, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, goal_id, amount_cents, occurred_on, note, transaction_id, created_by, created_at`,
		id, req.AmountCents, date, req.Note, userID,
	)
	c, err := scanContribution(row)
	if err != nil {
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if _, err := tx.Exec(`UPDATE savings_goals SET current_cents = current_cents + $1 WHERE id = $2`, req.AmountCents, id); err != nil {
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var transferID int64
	err = tx.QueryRow(
		`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, from_card_id)
		 VALUES ('transfer', $1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		req.AmountCents, transferCategory, "Aporte a meta: "+goalName, date, userID, defaultCardID,
	).Scan(&transferID)
	if err != nil {
		log.Printf("finances: create contribution transfer failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if _, err := tx.Exec(`UPDATE savings_contributions SET transaction_id = $1 WHERE id = $2`, transferID, c.ID); err != nil {
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	c.TransactionID = &transferID

	if err := tx.Commit(); err != nil {
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, c)
}
