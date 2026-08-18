package finances

import (
	"net/http"
	"testing"
)

// These cover the security gap fixed this slice: budgets, subscriptions,
// and categories had zero created_by scoping — any guest could read/edit/
// delete any other user's row. See budgets.go/subscriptions.go/
// categories.go for the scopeToOwner wiring these tests exercise.

func TestBudgets_GuestCannotSeeOrMutateAnotherUsersBudget(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	seedGuestUser(t, db)

	// Admin creates a budget for "comida".
	w := do(t, db, http.MethodPut, "/budgets/comida", map[string]any{"monthlyLimitCents": 50000})
	if w.Code != http.StatusOK {
		t.Fatalf("admin create budget: %d %s", w.Code, w.Body.String())
	}

	// Guest's own list shouldn't include admin's budget.
	w = doAs(t, db, 2, "guest", http.MethodGet, "/budgets", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("guest list budgets: %d %s", w.Code, w.Body.String())
	}
	var listResp listBudgetsResponse
	mustDecode(t, w, &listResp)
	for _, b := range listResp.Budgets {
		if b.Category == "comida" {
			t.Fatalf("guest saw admin's budget: %+v", b)
		}
	}

	// Guest deleting admin's budget 404s instead of succeeding.
	w = doAs(t, db, 2, "guest", http.MethodDelete, "/budgets/comida", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest delete admin's budget: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}

	// Admin's budget is untouched.
	w = do(t, db, http.MethodGet, "/budgets", nil)
	mustDecode(t, w, &listResp)
	found := false
	for _, b := range listResp.Budgets {
		if b.Category == "comida" {
			found = true
		}
	}
	if !found {
		t.Fatal("admin's budget was deleted by the guest's request")
	}
}

func TestBudgets_TwoUsersCanEachBudgetTheSameCategory(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	seedGuestUser(t, db)

	w := do(t, db, http.MethodPut, "/budgets/comida", map[string]any{"monthlyLimitCents": 50000})
	if w.Code != http.StatusOK {
		t.Fatalf("admin create budget: %d %s", w.Code, w.Body.String())
	}
	w = doAs(t, db, 2, "guest", http.MethodPut, "/budgets/comida", map[string]any{"monthlyLimitCents": 30000})
	if w.Code != http.StatusOK {
		t.Fatalf("guest create budget for same category: %d %s", w.Code, w.Body.String())
	}

	// Guest's own view is scoped to just their row — the real assertion for
	// "two users, one category, no conflict". Admin sees both (scopeToOwner's
	// existing "admin sees everything unscoped" convention, same as cards/
	// goals — not re-asserted here, that's covered by those modules' own
	// tests).
	w = doAs(t, db, 2, "guest", http.MethodGet, "/budgets", nil)
	var guestBudgets listBudgetsResponse
	mustDecode(t, w, &guestBudgets)
	if len(guestBudgets.Budgets) != 1 || guestBudgets.Budgets[0].MonthlyLimitCents != 30000 {
		t.Fatalf("guest budgets = %+v, want one row at 30000", guestBudgets.Budgets)
	}
}

func TestSubscriptions_GuestCannotSeeOrMutateAnotherUsersSubscription(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	seedGuestUser(t, db)
	cardID := insertCard(t, db, "debito")

	w := do(t, db, http.MethodPost, "/subscriptions", map[string]any{
		"name": "Netflix", "amountCents": 3990, "frequency": "monthly", "type": "expense",
		"category": "suscripciones", "nextBillingOn": "2026-09-01", "cardId": cardID, "active": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create subscription: %d %s", w.Code, w.Body.String())
	}
	var created Subscription
	mustDecode(t, w, &created)

	// Guest's list is empty — this subscription isn't theirs.
	w = doAs(t, db, 2, "guest", http.MethodGet, "/subscriptions", nil)
	var listResp listSubscriptionsResponse
	mustDecode(t, w, &listResp)
	if len(listResp.Subscriptions) != 0 {
		t.Fatalf("guest saw another user's subscriptions: %+v", listResp.Subscriptions)
	}

	// Guest can't delete it either.
	w = doAs(t, db, 2, "guest", http.MethodDelete, "/subscriptions/"+itoa(int(created.ID)), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("guest delete admin's subscription: status = %d, want 404 (body %s)", w.Code, w.Body.String())
	}
}

func TestCategories_GuestSeesSharedDefaultsButNotOtherUsersPrivateOnes(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	seedGuestUser(t, db)

	// Admin creates a private custom category.
	w := do(t, db, http.MethodPost, "/categories", map[string]any{
		"name": "mascota", "kind": "expense", "label": "Mascota",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create category: %d %s", w.Code, w.Body.String())
	}

	w = doAs(t, db, 2, "guest", http.MethodGet, "/categories", nil)
	var listResp listCategoriesResponse
	mustDecode(t, w, &listResp)
	sawSeeded, sawPrivate := false, false
	for _, c := range listResp.Categories {
		if c.Name == "comida" {
			sawSeeded = true
		}
		if c.Name == "mascota" {
			sawPrivate = true
		}
	}
	if !sawSeeded {
		t.Fatal("guest didn't see the shared seeded 'comida' category")
	}
	if sawPrivate {
		t.Fatal("guest saw admin's private 'mascota' category")
	}
}

func TestCategories_UnauthenticatedSeesOnlySharedDefaults(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)

	w := do(t, db, http.MethodPost, "/categories", map[string]any{
		"name": "mascota", "kind": "expense", "label": "Mascota",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create category: %d %s", w.Code, w.Body.String())
	}

	req, _ := http.NewRequest(http.MethodGet, "/categories", nil)
	w2 := recordNoAuth(db, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("unauthenticated GET /categories: status = %d, want 200 (body %s)", w2.Code, w2.Body.String())
	}
	var listResp listCategoriesResponse
	mustDecode(t, w2, &listResp)
	for _, c := range listResp.Categories {
		if c.Name == "mascota" {
			t.Fatal("unauthenticated caller saw a private category")
		}
	}
	if len(listResp.Categories) == 0 {
		t.Fatal("unauthenticated caller saw no categories at all — the public picker would be empty")
	}
}

// Regression for the bug this categories model originally shipped with: a
// single global UNIQUE(name, kind) made a category name exclusive across
// EVERY user even though visibility is per-user — one partner's private
// category silently blocked the other partner from ever creating their own
// same-named one, with no way to see what was blocking them. Fixed by
// splitting into two partial unique indexes (see migrate.go's comment on
// idx_categories_name_kind_owned/idx_categories_name_kind_shared).
func TestCategories_TwoUsersCanEachCreateTheSamePrivateName(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	seedGuestUser(t, db)

	w := do(t, db, http.MethodPost, "/categories", map[string]any{
		"name": "mascota", "kind": "expense", "label": "Mascota",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create 'mascota': %d %s", w.Code, w.Body.String())
	}

	// Same name, different (guest) owner — must succeed, not 409.
	w = doAs(t, db, 2, "guest", http.MethodPost, "/categories", map[string]any{
		"name": "mascota", "kind": "expense", "label": "Mascota",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("guest create their own 'mascota': status = %d, want 201 (body %s)", w.Code, w.Body.String())
	}

	// Each only sees their own.
	w = doAs(t, db, 2, "guest", http.MethodGet, "/categories", nil)
	var listResp listCategoriesResponse
	mustDecode(t, w, &listResp)
	count := 0
	for _, c := range listResp.Categories {
		if c.Name == "mascota" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("guest sees %d 'mascota' categories, want exactly 1 (their own, not admin's)", count)
	}
}

// The other half of the fix: creating a category that shadows an existing
// SHARED default (created_by IS NULL) must still be rejected — the caller
// can already see that one in their own list, so a silent duplicate would
// be confusing, not a fair "you can't see what's blocking you" case.
func TestCategories_CannotShadowASharedDefault(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)

	// "comida" is one of the 15 seeded defaults (migrate.go).
	w := do(t, db, http.MethodPost, "/categories", map[string]any{
		"name": "comida", "kind": "expense", "label": "Comida otra vez",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("create category shadowing a seeded default: status = %d, want 409 (body %s)", w.Code, w.Body.String())
	}
}

// Regression: GET /categories is mounted with zero middleware in a naive
// reading of routes.go's history — which means a request WITH a valid
// access_token cookie never got it parsed, so every caller (logged in or
// not) fell into the "anonymous" branch and only ever saw the 15 seeded
// defaults, never their own created ones. Reproduced here the way
// production actually invokes this route (no outer JWTAuth wrapper — see
// doDirectAs), which is what caught it; going through authHandler like
// every other test in this file would have hidden it.
func TestCategories_LoggedInCallerSeesOwnCreatedCategoryOnDirectRoute(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)
	seedGuestUser(t, db)

	w := doDirectAs(t, db, 1, "admin", http.MethodPost, "/categories", map[string]any{
		"name": "mascota", "kind": "expense", "label": "Mascota",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("admin create category: %d %s", w.Code, w.Body.String())
	}

	w = doDirectAs(t, db, 1, "admin", http.MethodGet, "/categories", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin list categories: %d %s", w.Code, w.Body.String())
	}
	var listResp listCategoriesResponse
	mustDecode(t, w, &listResp)
	found := false
	for _, c := range listResp.Categories {
		if c.Name == "mascota" {
			found = true
		}
	}
	if !found {
		t.Fatal("admin didn't see their own just-created category via GET /categories")
	}

	// Guest creates their own private one too, and sees it on the same
	// no-outer-wrapper path.
	w = doDirectAs(t, db, 2, "guest", http.MethodPost, "/categories", map[string]any{
		"name": "gimnasio", "kind": "expense", "label": "Gimnasio",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("guest create category: %d %s", w.Code, w.Body.String())
	}
	w = doDirectAs(t, db, 2, "guest", http.MethodGet, "/categories", nil)
	mustDecode(t, w, &listResp)
	found = false
	for _, c := range listResp.Categories {
		if c.Name == "gimnasio" {
			found = true
		}
	}
	if !found {
		t.Fatal("guest didn't see their own just-created category via GET /categories")
	}
}
