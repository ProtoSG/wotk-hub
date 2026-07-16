package finances

import (
	"database/sql"
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
	var defaultCardID sql.NullInt64
	err := row.Scan(&g.ID, &g.Name, &g.TargetCents, &g.CurrentCents, &deadline, &g.Icon, &g.Color, &g.DefaultCardID, &g.CreatedBy, &createdAt)
	if err != nil {
		return g, err
	}
	if deadline.Valid {
		g.Deadline = deadline.Time.Format(dateLayout)
	}
	if defaultCardID.Valid {
		g.DefaultCardID = &defaultCardID.Int64
	}
	g.CreatedAt = createdAt.Format(time.RFC3339)
	return g, nil
}

func scanContribution(row interface{ Scan(...any) error }) (SavingsContribution, error) {
	var c SavingsContribution
	var createdAt time.Time
	var occurredOn time.Time
	err := row.Scan(&c.ID, &c.GoalID, &c.AmountCents, &occurredOn, &c.Note, &c.CreatedBy, &createdAt)
	if err != nil {
		return c, err
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
		FROM savings_goals WHERE 1=1`
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"goals": goals})
}

// CreateGoal creates a new savings goal for the authenticated user.
func (h *handler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := middleware.UserFromContext(r.Context())
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

	row := h.db.QueryRow(
		`INSERT INTO savings_goals (name, target_cents, current_cents, deadline, icon, color, default_card_id, created_by)
		 VALUES ($1, $2, 0, $3, $4, $5, $6, $7)
		 RETURNING id, name, target_cents, current_cents, deadline, icon, color, default_card_id, created_by, created_at`,
		req.Name, req.TargetCents, req.Deadline, req.Icon, req.Color, req.DefaultCardID, userID,
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

	query := `UPDATE savings_goals
		SET name = $1, target_cents = $2, deadline = $3, icon = $4, color = $5, default_card_id = $6
		WHERE id = $7`
	args := []any{req.Name, req.TargetCents, req.Deadline, req.Icon, req.Color, req.DefaultCardID, id}
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

// DeleteGoal deletes a savings goal and its contributions.
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

	query := `DELETE FROM savings_goals WHERE id = $1`
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

// goalOwned checks that the goal exists and is owned by the caller.
func (h *handler) goalOwned(id int64, role string, userID int64) error {
	query := `SELECT id FROM savings_goals WHERE id = $1`
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
		`SELECT id, goal_id, amount_cents, occurred_on, note, created_by, created_at
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"contributions": contributions})
}

// CreateContribution atomically inserts a contribution and updates the goal's current_cents.
// If the goal has a default_card_id set, the contribution amount is also deducted from
// that card's balance_cents in the same transaction.
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
	if err := h.goalOwned(id, role, userID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "goal not found")
		return
	} else if err != nil {
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
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

	// Fetch the goal's default_card_id inside the transaction.
	var defaultCardID sql.NullInt64
	if err := tx.QueryRow(`SELECT default_card_id FROM savings_goals WHERE id = $1`, id).Scan(&defaultCardID); err != nil {
		tx.Rollback()
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	row := tx.QueryRow(
		`INSERT INTO savings_contributions (goal_id, amount_cents, occurred_on, note, created_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, goal_id, amount_cents, occurred_on, note, created_by, created_at`,
		id, req.AmountCents, date, req.Note, userID,
	)
	c, err := scanContribution(row)
	if err != nil {
		tx.Rollback()
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	if _, err := tx.Exec(`UPDATE savings_goals SET current_cents = current_cents + $1 WHERE id = $2`, req.AmountCents, id); err != nil {
		tx.Rollback()
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// If a default card is set, deduct the contribution amount from the card's balance.
	if defaultCardID.Valid {
		if _, err := tx.Exec(`UPDATE cards SET balance_cents = balance_cents - $1 WHERE id = $2`, req.AmountCents, defaultCardID.Int64); err != nil {
			tx.Rollback()
			log.Printf("finances: create contribution failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("finances: create contribution failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, c)
}
