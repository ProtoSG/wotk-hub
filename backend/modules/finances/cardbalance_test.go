package finances

import (
	"database/sql"
	"testing"
)

// cardBalance is the live-computed balance/used-credit for a card from its
// transactions. Under the mandatory-card model, income tagged to a
// non-credito card ADDS to its balance (the reversal of the old "income is
// invisible to cardBalance" behavior). Credito cards have no spendable
// balance — expenses only accrue used_credit. These tests exercise the SQL
// CASE directly so the income branch is pinned independent of routing.
func TestCardBalance_Case(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)

	var cardX, cardC int64
	if err := db.QueryRow(`INSERT INTO cards (name,bank,created_by) VALUES ('X','t',1) RETURNING id`).Scan(&cardX); err != nil {
		t.Fatalf("insert X: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO cards (name,bank,credit_limit_cents,created_by) VALUES ('C','t',1000000,1) RETURNING id`).Scan(&cardC); err != nil {
		t.Fatalf("insert C: %v", err)
	}

	// Seed X with a 1000 baseline via a transfer in (to_card_id branch).
	exec(t, db, `INSERT INTO transactions (type,amount_cents,category,description,occurred_on,to_card_id)
		VALUES ('transfer',1000,'transferencia','seed',CURRENT_DATE,$1)`, cardX)

	// The rows the CASE must react to. card_id is mandatory for
	// income/expense (CHECK enforces it), so every income/expense is tagged.
	exec(t, db, `INSERT INTO transactions (type,amount_cents,category,description,occurred_on,card_id)
		VALUES ('income', 200,'sueldo',  'income in',  CURRENT_DATE,$1)`, cardX) // X +200 (NEW branch)
	exec(t, db, `INSERT INTO transactions (type,amount_cents,category,description,occurred_on,card_id)
		VALUES ('expense',150,'comida',  'expense out',CURRENT_DATE,$1)`, cardX) // X -150
	exec(t, db, `INSERT INTO transactions (type,amount_cents,category,description,occurred_on,card_id)
		VALUES ('expense',500,'servicios','cred expense',CURRENT_DATE,$1)`, cardC) // C used_credit +500, balance untouched
	exec(t, db, `INSERT INTO transactions (type,amount_cents,category,description,occurred_on,from_card_id)
		VALUES ('transfer',100,'transferencia','out transfer',CURRENT_DATE,$1)`, cardX) // X -100 (from_card_id branch)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	bal, used, err := cardBalance(tx, cardX, 0)
	if err != nil {
		t.Fatalf("cardBalance(X): %v", err)
	}
	// 1000 (seed) +200 (income, NEW) -150 (expense) -100 (transfer out) = 950.
	const wantX = int64(1000 + 200 - 150 - 100)
	if bal != wantX {
		t.Fatalf("cardBalance(X) = %d want %d (income branch absent or wrong)", bal, wantX)
	}
	if used != 0 {
		t.Fatalf("used_credit(X) = %d want 0", used)
	}

	balC, usedC, err := cardBalance(tx, cardC, 0)
	if err != nil {
		t.Fatalf("cardBalance(C): %v", err)
	}
	// Credito balance is never credited by income; its only ledger effect
	// is used_credit from expenses.
	if balC != 0 {
		t.Fatalf("cardBalance(C) = %d want 0 (credito has no spendable balance)", balC)
	}
	if usedC != 500 {
		t.Fatalf("used_credit(C) = %d want 500", usedC)
	}
}

func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}
