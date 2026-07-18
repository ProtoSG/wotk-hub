package finances

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The authHandler/do() helpers in finances_test.go wrap Routes in an outer
// middleware.JWTAuth, so every request — including the public /categories
// route — has always gone through that outer JWTAuth first. These tests
// hit Routes directly, unwrapped, to actually exercise the public/protected
// split RequireAuth is responsible for inside the router itself.

func TestListCategories_PublicWithoutAuth(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)

	r := httptest.NewRequest(http.MethodGet, "/categories", nil)
	w := httptest.NewRecorder()

	Routes(db, testJWTSecret).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
}

func TestListTransactions_RequiresAuth(t *testing.T) {
	db := setupTestDB(t)
	resetFinanceTables(t, db)

	r := httptest.NewRequest(http.MethodGet, "/transactions", nil)
	w := httptest.NewRecorder()

	Routes(db, testJWTSecret).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %s)", w.Code, w.Body.String())
	}
}
