package finances

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// R2 carve-out (design #40 / SPEC decision log): subscriptions CAN target a
// credito card. processDue writes an expense (not an inflow), and
// expense-to-credito is unchanged by the mandatory-card model. The two
// tests below are the positive-path counterpart to the credito inflow
// guards on CreateTransaction (income) and CreateCardTransfer (both
// sides): credito is blocked for income/transfer, NOT for subscriptions.

// Slice 1b — subscriptions: subscriptionRequest.CardID becomes int64
// (mandatory, validated "cardId requerido"); Create/Update always call
// subscriptionCardExists; processDue's INSERT always binds the subscription's
// card_id (no NULL path — backed by the subscriptions NOT NULL constraint
// from slice 1a). Reuses the slice-1a test harness.

// TestCreateSubscription_RequiresCardID — CardID is now a non-optional int64;
// missing or <=0 is rejected before any DB write with "cardId requerido".
func TestCreateSubscription_RequiresCardID(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing cardId rejected",
			body: map[string]any{"name": "Netflix", "amountCents": 1000, "frequency": "monthly", "category": "suscripciones", "nextBillingOn": "2026-08-01"},
		},
		{
			name: "cardId=0 rejected",
			body: map[string]any{"name": "Netflix", "amountCents": 1000, "frequency": "monthly", "category": "suscripciones", "nextBillingOn": "2026-08-01", "cardId": 0},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := do(t, db, http.MethodPost, "/subscriptions", c.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d want 400 (body %s)", w.Code, w.Body.String())
			}
			if got := errBody(w); got != "cardId requerido" {
				t.Fatalf("error message = %q want %q", got, "cardId requerido")
			}
		})
	}

	// Sanity: a valid subscription WITH a card is accepted.
	w := do(t, db, http.MethodPost, "/subscriptions", map[string]any{
		"name": "ok", "amountCents": 1000, "frequency": "monthly", "category": "suscripciones", "nextBillingOn": "2026-08-01", "cardId": deb,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("valid subscription status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
}

// TestCreateSubscription_NonExistentCardRejected — cardId is mandatory AND
// must reference a real, non-archived card. A bogus cardId surfaces as 404
// (consistent with the validate-then-check-existence ordering).
func TestCreateSubscription_NonExistentCardRejected(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	w := do(t, db, http.MethodPost, "/subscriptions", map[string]any{
		"name": "x", "amountCents": 1000, "frequency": "monthly", "category": "suscripciones", "nextBillingOn": "2026-08-01", "cardId": 999999,
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d want 404 for non-existent card (body %s)", w.Code, w.Body.String())
	}
}

// TestUpdateSubscription_RequiresCardID — same invariant on update.
func TestUpdateSubscription_RequiresCardID(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")
	// Seed a subscription to update.
	w := do(t, db, http.MethodPost, "/subscriptions", map[string]any{
		"name": "orig", "amountCents": 1000, "frequency": "monthly", "category": "suscripciones", "nextBillingOn": "2026-08-01", "cardId": deb,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("seed subscription status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created subscription: %v", err)
	}

	// Update without cardId → 400 "cardId requerido".
	w = do(t, db, http.MethodPut, "/subscriptions/"+itoa(int(created.ID)), map[string]any{
		"name": "renamed", "amountCents": 2000, "frequency": "monthly", "category": "suscripciones", "nextBillingOn": "2026-09-01",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("update without cardId status = %d want 400 (body %s)", w.Code, w.Body.String())
	}
	if got := errBody(w); got != "cardId requerido" {
		t.Fatalf("error message = %q want %q", got, "cardId requerido")
	}
}

// TestProcessDue_AlwaysTaggedCardID — processDue's expense INSERT always
// binds the subscription's card_id (never NULL). With subscriptions.card_id
// NOT NULL enforced by the slice-1a migration, every due charge carries a
// real card_id, so "no untagged money" holds for the auto-charge path.
func TestProcessDue_AlwaysTaggedCardID(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")
	cred := insertCard(t, db, "credito")

	// Charge both a debito- and a credito-tagged subscription to confirm
	// processDue writes a valid card_id in both cases (credito IS allowed as
	// an auto-charge target — processDue writes an expense, not an inflow).
	pastDate := time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	for _, cardID := range []int64{deb, cred} {
		if _, err := db.Exec(
			`INSERT INTO subscriptions (name, amount_cents, frequency, category, next_billing_on, active, card_id)
			 VALUES ($1, 100, 'monthly', 'suscripciones', $2, true, $3)`,
			"sub-"+itoa(int(cardID)), pastDate, cardID,
		); err != nil {
			t.Fatalf("seed subscription for card %d: %v", cardID, err)
		}
	}

	if err := ProcessDueSubscriptions(db); err != nil {
		t.Fatalf("processDue: %v", err)
	}

	rows, err := db.Query(
		`SELECT card_id, type FROM transactions
		 WHERE deleted_at IS NULL AND type='expense' AND description LIKE '%(suscripción)'
		 ORDER BY card_id`)
	if err != nil {
		t.Fatalf("query charged expenses: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cardID int64
		var typ string
		if err := rows.Scan(&cardID, &typ); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if cardID == 0 {
			t.Fatal("processDue wrote an expense with NULL/0 card_id — every due must carry a valid card")
		}
		if cardID != deb && cardID != cred {
			t.Fatalf("charged expense card_id=%d, but only deb=%d and cred=%d are seeded", cardID, deb, cred)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
}

// TestCreateSubscription_CreditoCardAllowed — R2 carve-out, positive path:
// a subscription created with a credito card succeeds (201). This is the
// contract that contrasts with TestCreateTransaction_IncomeToCreditCardRejected
// and TestCreateCardTransfer_CreditoRejected/{source,destination}: credito
// is blocked for income/transfer inflows, NOT for subscriptions
// (processDue charges as expense, allowed on credito).
func TestCreateSubscription_CreditoCardAllowed(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	cred := insertCard(t, db, "credito")

	w := do(t, db, http.MethodPost, "/subscriptions", map[string]any{
		"name": "Spotify", "amountCents": 1099, "frequency": "monthly",
		"category": "suscripciones", "nextBillingOn": "2026-08-01", "cardId": cred,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create credito subscription status = %d want 201 (body %s) — R2 carve-out: credito IS allowed here", w.Code, w.Body.String())
	}
	var s struct {
		ID     int64  `json:"id"`
		CardID *int64 `json:"cardId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode created subscription: %v", err)
	}
	if s.CardID == nil || *s.CardID != cred {
		t.Fatalf("created subscription cardId = %v, want %d (credito IS allowed for subscriptions)", s.CardID, cred)
	}
}

// TestProcessDue_CreditoSubscriptionEmitsExpense — R2 carve-out on the
// auto-charge path: processDue on a credito-tagged subscription does NOT
// trip rejectCreditCardForInflow (it writes an expense, not an inflow).
// The emitted expense is tagged to the credito card with a valid card_id,
// so "no untagged money" holds for the auto-charge path even when the
// target is credito. A future regression that adds a credito guard to
// processDue would trip this test intentionally.
func TestProcessDue_CreditoSubscriptionEmitsExpense(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	cred := insertCard(t, db, "credito")

	// Past nextBillingOn so processDue fires immediately.
	pastDate := time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	w := do(t, db, http.MethodPost, "/subscriptions", map[string]any{
		"name": "Netflix", "amountCents": 1500, "frequency": "monthly",
		"category": "suscripciones", "nextBillingOn": pastDate, "cardId": cred,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create credito subscription status = %d want 201 (body %s) — R2 carve-out", w.Code, w.Body.String())
	}

	if err := ProcessDueSubscriptions(db); err != nil {
		t.Fatalf("processDue: %v", err)
	}

	rows, err := db.Query(
		`SELECT id, card_id FROM transactions
		 WHERE deleted_at IS NULL AND type='expense'
		   AND card_id=$1 AND description LIKE '%(suscripción)'`,
		cred,
	)
	if err != nil {
		t.Fatalf("query emitted credito expenses: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
		var id, cardID int64
		if err := rows.Scan(&id, &cardID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if cardID != cred {
			t.Fatalf("emitted credito-subscription expense card_id=%d, want %d", cardID, cred)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if n == 0 {
		t.Fatal("processDue emitted 0 credito-tagged expenses — R2 carve-out broken (expected ≥1 expense tagged to the credito card)")
	}
}
