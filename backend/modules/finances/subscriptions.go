package finances

import (
	"database/sql"
	"log"
	"net/http"
	"time"
	"workhub/httpx"
)

func scanSubscription(row interface{ Scan(...any) error }) (Subscription, error) {
	var s Subscription
	var nextBilling, createdAt time.Time
	var cardID sql.NullInt64
	err := row.Scan(&s.ID, &s.Name, &s.AmountCents, &s.Frequency, &s.Category, &nextBilling, &createdAt, &s.Active, &cardID)
	if err != nil {
		return s, err
	}
	if cardID.Valid {
		s.CardID = &cardID.Int64
	}
	s.NextBillingOn = nextBilling.Format(dateLayout)
	s.CreatedAt = createdAt.Format(time.RFC3339)
	return s, nil
}

// advance moves a billing date forward by one period.
func advance(d time.Time, frequency string) time.Time {
	switch frequency {
	case "weekly":
		return d.AddDate(0, 0, 7)
	case "yearly":
		return d.AddDate(1, 0, 0)
	default: // monthly
		return d.AddDate(0, 1, 0)
	}
}

// ProcessDueSubscriptions charges due subscriptions: for each active
// subscription whose billing date has arrived, it inserts one expense
// transaction per billed period and advances next_billing_on past today.
//
// This used to run lazily as a side effect of every finances GET request
// (List/Summary/Budgets). It's now driven by a ticker in main() instead, so
// reads stay read-only and this write path runs on a predictable schedule.
// A single DB transaction with row locks (FOR UPDATE) still prevents
// duplicate charges if a tick and a manual trigger were ever to overlap.
func ProcessDueSubscriptions(db *sql.DB) error {
	h := &handler{db: db}
	return h.processDue()
}

func (h *handler) processDue() error {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT id, name, amount_cents, frequency, category, next_billing_on, card_id
		 FROM subscriptions
		 WHERE active AND next_billing_on <= $1
		 FOR UPDATE`, today)
	if err != nil {
		return err
	}

	type due struct {
		id          int64
		name        string
		amountCents int64
		frequency   string
		category    string
		next        time.Time
		cardID      int64 // NOT NULL in the DB since slice 1a — no Null path here
	}
	var dues []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.name, &d.amountCents, &d.frequency, &d.category, &d.next, &d.cardID); err != nil {
			rows.Close()
			return err
		}
		dues = append(dues, d)
	}
	rows.Close()

	// No balance check here on purpose: this is an unattended auto-charge,
	// not a user-initiated expense. A real subscription doesn't ask
	// permission when the linked account is short — it charges, and the
	// account goes negative. Blocking it here would silently skip a
	// commitment that already happened while still advancing
	// next_billing_on, which is worse than a card balance going negative.
	//
	// card_id is always bound: subscriptions.card_id is NOT NULL (slice 1a
	// migration) and Create/Update enforce a valid cardId, so every due
	// charge tags a real card — "no untagged money" holds for this path.
	for _, d := range dues {
		next := d.next
		for !next.After(today) {
			if _, err := tx.Exec(
				`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, card_id)
				 VALUES ('expense', $1, $2, $3, $4, $5)`,
				d.amountCents, d.category, d.name+" (suscripción)", next, d.cardID); err != nil {
				return err
			}
			next = advance(next, d.frequency)
		}
		if _, err := tx.Exec(`UPDATE subscriptions SET next_billing_on = $1 WHERE id = $2`, next, d.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// monthlyCents normalizes a subscription cost to a monthly amount in cents.
func monthlyCents(amountCents int64, frequency string) int64 {
	switch frequency {
	case "weekly":
		return amountCents * 52 / 12
	case "yearly":
		return amountCents / 12
	default:
		return amountCents
	}
}

// ListSubscriptions returns every subscription along with the total
// monthly-normalized committed spend across active ones.
//
// @Summary List subscriptions
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Success 200 {object} listSubscriptionsResponse
// @Router /finances/subscriptions [get]
func (h *handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(
		`SELECT id, name, amount_cents, frequency, category, next_billing_on, created_at, active, card_id
		 FROM subscriptions ORDER BY next_billing_on ASC, id ASC`)
	if err != nil {
		log.Printf("finances: list subscriptions failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	defer rows.Close()

	subscriptions := []Subscription{}
	var committed int64
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			log.Printf("finances: scan subscription failed: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
			return
		}
		if s.Active {
			committed += monthlyCents(s.AmountCents, s.Frequency)
		}
		subscriptions = append(subscriptions, s)
	}
	httpx.WriteJSON(w, http.StatusOK, listSubscriptionsResponse{
		Subscriptions:         subscriptions,
		MonthlyCommittedCents: committed,
	})
}

// subscriptionCardExists checks the card exists and isn't archived.
// Unscoped by owner — subscriptions have no ownership model of their own
// (see ListSubscriptions/UpdateSubscription/DeleteSubscription, none of
// which check created_by either), so gatekeeping just this field would be
// an inconsistent, partial fix to that larger gap rather than this change.
func (h *handler) subscriptionCardExists(cardID int64) error {
	var got int64
	return h.db.QueryRow(`SELECT id FROM cards WHERE id = $1 AND deleted_at IS NULL`, cardID).Scan(&got)
}

// CreateSubscription creates a recurring subscription tied to a card.
//
// @Summary Create a subscription
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body subscriptionRequest true "Subscription details"
// @Success 201 {object} Subscription
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError "card not found"
// @Router /finances/subscriptions [post]
func (h *handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req subscriptionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	next, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	if err := h.categoryExists(req.Category, "expense"); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid category: "+req.Category)
		return
	} else if err != nil {
		log.Printf("finances: create subscription category check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	// CardId is mandatory (see validate); always confirm the card exists
	// and isn't archived before opening the write. A bogus or unowned card
	// surfaces as a clean 404, not a FK violation 500.
	if err := h.subscriptionCardExists(req.CardID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: create subscription card check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	row := h.db.QueryRow(
		`INSERT INTO subscriptions (name, amount_cents, frequency, category, next_billing_on, active, card_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, name, amount_cents, frequency, category, next_billing_on, created_at, active, card_id`,
		req.Name, req.AmountCents, req.Frequency, req.Category, next, req.isActive(), req.CardID,
	)
	s, err := scanSubscription(row)
	if err != nil {
		log.Printf("finances: create subscription failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, s)
}

// UpdateSubscription updates an existing subscription. Reactivating a
// paused subscription skips missed billing periods without back-charging.
//
// @Summary Update a subscription
// @Tags finances
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "Subscription ID"
// @Param body body subscriptionRequest true "Subscription details"
// @Success 200 {object} Subscription
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /finances/subscriptions/{id} [put]
func (h *handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	var req subscriptionRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	next, err := req.validate()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	if err := h.categoryExists(req.Category, "expense"); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid category: "+req.Category)
		return
	} else if err != nil {
		log.Printf("finances: update subscription category check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if err := h.subscriptionCardExists(req.CardID); err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "card not found")
		return
	} else if err != nil {
		log.Printf("finances: update subscription card check failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	var wasActive bool
	err = h.db.QueryRow(`SELECT active FROM subscriptions WHERE id = $1`, id).Scan(&wasActive)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "subscription not found")
		return
	}
	if err != nil {
		log.Printf("finances: lookup subscription failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}

	// Reactivation is a fresh start: skip missed periods without charging.
	// The billing date silently advances past today so processDue does not
	// back-charge the months the subscription was paused.
	if !wasActive && req.isActive() {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		for !next.After(today) {
			next = advance(next, req.Frequency)
		}
	}

	row := h.db.QueryRow(
		`UPDATE subscriptions
		 SET name = $1, amount_cents = $2, frequency = $3, category = $4, next_billing_on = $5, active = $6, card_id = $7
		 WHERE id = $8
		 RETURNING id, name, amount_cents, frequency, category, next_billing_on, created_at, active, card_id`,
		req.Name, req.AmountCents, req.Frequency, req.Category, next, req.isActive(), req.CardID, id,
	)
	s, err := scanSubscription(row)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "subscription not found")
		return
	}
	if err != nil {
		log.Printf("finances: update subscription failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

// DeleteSubscription hard-deletes a subscription.
//
// @Summary Delete a subscription
// @Tags finances
// @Produce json
// @Security CookieAuth
// @Param id path int true "Subscription ID"
// @Success 200 {object} httpx.SuccessResponse
// @Failure 400 {object} httpx.APIError
// @Failure 404 {object} httpx.APIError
// @Router /finances/subscriptions/{id} [delete]
func (h *handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	res, err := h.db.Exec(`DELETE FROM subscriptions WHERE id = $1`, id)
	if err != nil {
		log.Printf("finances: delete subscription failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "subscription not found")
		return
	}
	httpx.WriteSuccess(w, http.StatusOK)
}
