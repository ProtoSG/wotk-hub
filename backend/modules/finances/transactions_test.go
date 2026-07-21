package finances

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Mandatory-card model, HTTP layer. These cover the acceptance scenarios
// from the spec that live behind a real request: cardId required on
// create (income + expense), income to a non-credito card moves its
// balance, income to a credito card is rejected, and refund repone the
// card's balance (the atomic pair — the cardBalance income branch makes a
// refund's compensating income row actually credit the card).

func TestCreateTransaction_RequiresCardID(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "income without cardId rejected",
			body: map[string]any{"type": "income", "amountCents": 100, "category": "sueldo", "description": "x", "date": "2026-07-17"},
		},
		{
			name: "expense without cardId rejected",
			body: map[string]any{"type": "expense", "amountCents": 100, "category": "comida", "description": "x", "date": "2026-07-17"},
		},
		{
			name: "income with cardId=0 rejected",
			body: map[string]any{"type": "income", "amountCents": 100, "category": "sueldo", "description": "x", "date": "2026-07-17", "cardId": 0},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := do(t, db, http.MethodPost, "/transactions", c.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d want 400 (body %s)", w.Code, w.Body.String())
			}
			if got := errBody(w); got != "cardId requerido" {
				t.Fatalf("error message = %q want %q", got, "cardId requerido")
			}
		})
	}

	// Sanity: the same income WITH a valid cardId is accepted — proves the
	// 400 above is about cardId, not some other field.
	_ = deb
	w := do(t, db, http.MethodPost, "/transactions", map[string]any{
		"type": "income", "amountCents": 100, "category": "sueldo", "description": "ok", "date": "2026-07-17", "cardId": deb,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("valid income status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
}

func TestCreateTransaction_IncomeMovesCardBalance(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")
	seedCardBalance(t, db, deb, 500) // baseline 500 via a transfer in

	w := do(t, db, http.MethodPost, "/transactions", map[string]any{
		"type": "income", "amountCents": 100, "category": "sueldo", "description": "salario", "date": "2026-07-17", "cardId": deb,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create income status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
	if got := cardBalanceJSON(t, db, deb); got != 600 {
		t.Fatalf("card balance after income = %d want 600 (income must move the balance)", got)
	}

	// An expense draws the balance back down — proves the CASE handles both
	// branches on the same card, not just income.
	w = do(t, db, http.MethodPost, "/transactions", map[string]any{
		"type": "expense", "amountCents": 200, "category": "comida", "description": "almuerzo", "date": "2026-07-17", "cardId": deb,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create expense status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
	if got := cardBalanceJSON(t, db, deb); got != 400 {
		t.Fatalf("card balance after expense = %d want 400", got)
	}
}

func TestCreateTransaction_IncomeToCreditCardRejected(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	cred := insertCard(t, db, "credito")

	w := do(t, db, http.MethodPost, "/transactions", map[string]any{
		"type": "income", "amountCents": 100, "category": "sueldo", "description": "a credito", "date": "2026-07-17", "cardId": cred,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d want 400 (body %s)", w.Code, w.Body.String())
	}
	if got := errBody(w); got != errCreditInflow.Error() {
		t.Fatalf("error message = %q want %q", got, errCreditInflow.Error())
	}
	// Credito balance and used_credit stay at zero — no income row leaked.
	var tagged int64
	if err := db.QueryRow(
		`SELECT COALESCE((SELECT SUM(amount_cents) FROM transactions
		  WHERE deleted_at IS NULL AND card_id=$1 AND type='income'),0)
		 FROM cards WHERE id=$1`, cred,
	).Scan(&tagged); err != nil {
		t.Fatalf("check cred income: %v", err)
	}
	if tagged != 0 {
		t.Fatalf("credito income row leaked: %d cents tagged", tagged)
	}
}

// TestRefundTransaction_ReponeBalance is the atomic pair with the income
// branch: refunding an expense tagged to a debito card inserts an income
// row (cardBalance's new branch credits it), so the card's balance returns
// toward its pre-expense value. RefundTransaction code is UNCHANGED — the
// new cardBalance income branch is what makes refund repone.
func TestRefundTransaction_ReponeBalance(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")
	seedCardBalance(t, db, deb, 500) // baseline 500

	// Expense 80 → balance 420.
	w := do(t, db, http.MethodPost, "/transactions", map[string]any{
		"type": "expense", "amountCents": 80, "category": "comida", "description": "almuerzo", "date": "2026-07-17", "cardId": deb,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create expense status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode expense: %v", err)
	}
	if got := cardBalanceJSON(t, db, deb); got != 420 {
		t.Fatalf("balance after expense = %d want 420", got)
	}

	// Refund: inserts an income row tagged to deb. With the income branch,
	// balance returns to 500.
	w = do(t, db, http.MethodPost, "/transactions/"+itoa(int(created.ID))+"/refund", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("refund status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
	if got := cardBalanceJSON(t, db, deb); got != 500 {
		t.Fatalf("balance after refund = %d want 500 (refund must repone saldo)", got)
	}
}

// TestCreateTransaction_NonOwnedCardRejected — a card belonging to nobody
// (unseeded id) surfaces as a clean 404, not a 500 from a FK violation.
func TestCreateTransaction_NonOwnedCardRejected(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	w := do(t, db, http.MethodPost, "/transactions", map[string]any{
		"type": "income", "amountCents": 100, "category": "sueldo", "description": "x", "date": "2026-07-17", "cardId": 999999,
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d want 404 for non-owned card (body %s)", w.Code, w.Body.String())
	}
}

// TestRefundTransaction_CreditoExpenseBalanceUnchanged locks the documented
// ACCEPTED edge (design #40 / SPEC decision log): refunding an expense
// originally tagged to a credito card INSERTs a new income row tagged to
// that same credito card_id, but the cardBalance CASE's income branch is
// guarded by `card_type != 'credito'`, so the credito card's computed
// balance (and used_credit) stays UNCHANGED before vs after the refund.
// The real fix (reducing used_credit on a credito-expense refund) is
// deferred to the future credit_lines split; this test keeps the current
// behavior pinned so a future change to the CASE trips it intentionally
// rather than silently.
func TestRefundTransaction_CreditoExpenseBalanceUnchanged(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	cred := insertCard(t, db, "credito")

	// Credito accepts an expense (no inflow guard on expense). Its balance
	// stays 0 (credito has no spendable balance — only used_credit moves).
	w := do(t, db, http.MethodPost, "/transactions", map[string]any{
		"type": "expense", "amountCents": 300, "category": "comida",
		"description": "cena credito", "date": "2026-07-17", "cardId": cred,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create credito expense status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode expense: %v", err)
	}

	// Baseline: balance 0, used_credit 300.
	balanceBefore, usedBefore := cardStateJSON(t, db, cred)
	if balanceBefore != 0 {
		t.Fatalf("credito balance after expense = %d want 0 (credito has no spendable balance)", balanceBefore)
	}
	if usedBefore != 300 {
		t.Fatalf("credito used_credit after expense = %d want 300", usedBefore)
	}

	// Refund the credito expense. Expects 201 (RefundTransaction only
	// rejects non-expense rows; credito cards are not gated here — the
	// inflow helper is NOT applied to refunds per design #40).
	w = do(t, db, http.MethodPost, "/transactions/"+itoa(int(created.ID))+"/refund", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("refund credito expense status = %d want 201 (body %s)", w.Code, w.Body.String())
	}

	// (a) The refund INSERTed a new income row tagged to the credito card.
	var refundCardID int64
	err := db.QueryRow(
		`SELECT card_id FROM transactions
		 WHERE deleted_at IS NULL AND type='income'
		   AND card_id=$1 AND description LIKE 'Reembolso: %'
		 ORDER BY id DESC LIMIT 1`,
		cred,
	).Scan(&refundCardID)
	if err != nil {
		t.Fatalf("querying refund income row tagged to credito card: %v (a refund income row tagged to the credito card must exist)", err)
	}
	if refundCardID != cred {
		t.Fatalf("refund income row card_id=%d, want %d (must be tagged to the credito card)", refundCardID, cred)
	}

	// (b) Credito card's computed balance is UNCHANGED before vs after the
	// refund — the income branch correctly skips credito. used_credit is
	// also unchanged because the refund row is type='income', not 'expense'
	// (the used-credit SUM only credits credito expenses).
	balanceAfter, usedAfter := cardStateJSON(t, db, cred)
	if balanceAfter != balanceBefore {
		t.Fatalf("credito balance changed after refund: before=%d after=%d — income branch must skip credito", balanceBefore, balanceAfter)
	}
	if usedAfter != usedBefore {
		t.Fatalf("credito used_credit changed after refund: before=%d after=%d — refund is income, not expense; used_credit must not reduce", usedBefore, usedAfter)
	}
}

// --- DB-level CHECK constraint (independent of routing) ---

// The CHECK on transactions.card_id is the DB backstop behind validate: it
// must reject NULL card_id income/expense inserts and allow transfer rows
// with a NULL card_id (transfers use from_/to_card_id instead).
func TestTransactionsCheckConstraint(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	var cardA int64
	if err := db.QueryRow(`INSERT INTO cards (name,bank,created_by) VALUES ('A','t',1) RETURNING id`).Scan(&cardA); err != nil {
		t.Fatalf("insert A: %v", err)
	}

	t.Run("income NULL card_id rejected by CHECK", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO transactions (type,amount_cents,category,description,occurred_on) VALUES ('income',100,'sueldo','no card',CURRENT_DATE)`)
		if err == nil {
			t.Fatal("expected CHECK violation, got nil")
		}
	})
	t.Run("expense NULL card_id rejected by CHECK", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO transactions (type,amount_cents,category,description,occurred_on) VALUES ('expense',100,'comida','no card',CURRENT_DATE)`)
		if err == nil {
			t.Fatal("expected CHECK violation, got nil")
		}
	})
	t.Run("transfer NULL card_id allowed", func(t *testing.T) {
		res, err := db.Exec(`INSERT INTO transactions (type,amount_cents,category,description,occurred_on,to_card_id) VALUES ('transfer',100,'transferencia','seed',CURRENT_DATE,$1)`, cardA)
		if err != nil {
			t.Fatalf("transfer insert with NULL card_id failed: %v", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			t.Fatalf("transfer insert affected %d rows, want 1", n)
		}
	})
	t.Run("subscriptions card_id NOT NULL enforced", func(t *testing.T) {
		// processDue's expense insert binds subscriptions.card_id (made
		// NOT NULL by migration); a subscription insert without a card
		// must fail at the DB even before slice 1b's handler guard.
		_, err := db.Exec(`INSERT INTO subscriptions (name,amount_cents,frequency,category,next_billing_on,active) VALUES ('s',100,'monthly','comida',CURRENT_DATE,true)`)
		if err == nil {
			t.Fatal("expected NOT NULL violation on subscriptions.card_id, got nil")
		}
	})
}
