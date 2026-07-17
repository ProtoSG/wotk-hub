package finances

import (
	"net/http"
	"testing"
)

// Slice 1b — cards: delete-last-active-card invariant (409), reload
// endpoints removed (404 — route gone), and CreateCardTransfer credito
// rejection now goes through the shared rejectCreditCardForInflow helper
// (applies to both sides). Reuses the slice-1a test harness.

// TestDeleteCard_LastActiveCardRejected exercises the invariant added from
// scratch (explore R4 confirmed no prior delete rule of any kind): archiving
// the owner's only active card must be a 409, not a soft-delete.
func TestDeleteCard_LastActiveCardRejected(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	only := insertCard(t, db, "debito")

	w := do(t, db, http.MethodDelete, "/cards/"+itoa(int(only)), nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("delete only card status = %d want 409 (body %s)", w.Code, w.Body.String())
	}
	if got := errBody(w); got != "no podés archivar tu última tarjeta activa" {
		t.Fatalf("error message = %q want %q", got, "no podés archivar tu última tarjeta activa")
	}

	// The card must still exist (no soft-delete happened) — still listed.
	if got := cardBalanceJSON(t, db, only); got != 0 {
		// balance is 0 (no seed), this just asserts it's still there.
		t.Fatalf("last card disappeared after 409; expected to still be listed")
	}
}

// TestDeleteCard_TwoCardsDeletesOne — with two active cards, deleting one
// succeeds (200) and the other remains.
func TestDeleteCard_TwoCardsDeletesOne(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	a := insertCard(t, db, "debito")
	b := insertCard(t, db, "prepago")

	w := do(t, db, http.MethodDelete, "/cards/"+itoa(int(a)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete one of two cards status = %d want 200 (body %s)", w.Code, w.Body.String())
	}

	// The remaining card is still listed.
	_ = cardBalanceJSON(t, db, b) // asserts b is in the list

	// Now b is the last active one — deleting it must 409.
	w = do(t, db, http.MethodDelete, "/cards/"+itoa(int(b)), nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("delete last remaining card status = %d want 409 (body %s)", w.Code, w.Body.String())
	}
}

// TestReloadEndpointsRemoved — the reload routes (routes.go :33-34) are gone,
// so GET/POST /cards/{id}/reloads now hit chi's 404 handler, not a handler
// that returns 200/201.
func TestReloadEndpointsRemoved(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")

	t.Run("list reloads gone", func(t *testing.T) {
		w := do(t, db, http.MethodGet, "/cards/"+itoa(int(deb))+"/reloads", nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("GET /cards/{id}/reloads status = %d want 404 (route should be unregistered)", w.Code)
		}
	})
	t.Run("create reload gone", func(t *testing.T) {
		w := do(t, db, http.MethodPost, "/cards/"+itoa(int(deb))+"/reloads", map[string]any{
			"amountCents": 100, "date": "2026-07-17", "note": "x",
		})
		if w.Code != http.StatusNotFound {
			t.Fatalf("POST /cards/{id}/reloads status = %d want 404 (route should be unregistered)", w.Code)
		}
	})
}

// TestCreateCardTransfer_CreditoRejected — both sides reject a credito card
// via the shared rejectCreditCardForInflow helper (replaces the old inline
// "solo se puede transferir entre débito/prepago" guard).
func TestCreateCardTransfer_CreditoRejected(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	deb := insertCard(t, db, "debito")
	cred := insertCard(t, db, "credito")
	seedCardBalance(t, db, deb, 1000)

	cases := []struct {
		name       string
		from, to   int64
		wantSubstr string
	}{
		{name: "credito as source rejected", from: cred, to: deb, wantSubstr: errCreditInflow.Error()},
		{name: "credito as destination rejected", from: deb, to: cred, wantSubstr: errCreditInflow.Error()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := do(t, db, http.MethodPost, "/cards/transfers", map[string]any{
				"fromCardId": c.from, "toCardId": c.to, "amountCents": 100, "date": "2026-07-17", "note": "x",
			})
			if w.Code != http.StatusBadRequest {
				t.Fatalf("transfer with credito status = %d want 400 (body %s)", w.Code, w.Body.String())
			}
			if got := errBody(w); got != c.wantSubstr {
				t.Fatalf("error message = %q want %q", got, c.wantSubstr)
			}
		})
	}

	// Sanity: a debito→debito transfer still works (proves the credito
	// guard is the rejection, not something else).
	deb2 := insertCard(t, db, "debito")
	w := do(t, db, http.MethodPost, "/cards/transfers", map[string]any{
		"fromCardId": deb, "toCardId": deb2, "amountCents": 100, "date": "2026-07-17", "note": "ok",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("debito→debito transfer status = %d want 201 (body %s)", w.Code, w.Body.String())
	}
}
