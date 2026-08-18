package permissions

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
	"workhub/middleware"
	"workhub/store"

	"github.com/golang-jwt/jwt/v5"

	_ "github.com/lib/pq"
)

// Same harness shape as finances/gym (see their _test.go files) — one real
// Postgres instance, migrated schema, JWT cookie auth.

const testJWTSecret = "permissions-test-secret"

func testDSN() string {
	if v := os.Getenv("FINANCES_TEST_DB"); v != "" {
		return v
	}
	return "postgres://workhub:workhub@localhost:5432/workhub_test?sslmode=disable"
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", testDSN())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("test postgres unreachable (%v); set FINANCES_TEST_DB to run integration tests", err)
	}
	if err := store.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func resetTables(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`TRUNCATE TABLE users, module_permissions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO users (id, email, password_hash, name, role) VALUES
		 (1, 'admin@test', 'x', 'Admin', 'admin'),
		 (2, 'guest@test', 'x', 'Guest', 'guest')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
}

func testJWTFor(t *testing.T, userID int64, role string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10), "role": role, "exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return s
}

func authHandler(db *sql.DB) http.Handler {
	return middleware.JWTAuth(testJWTSecret)(Routes(db))
}

func doAs(t *testing.T, db *sql.DB, userID int64, role, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = httptest.NewRequest(method, path, bytes.NewReader(buf))
		r.Header.Set("Content-Type", "application/json")
	}
	r.AddCookie(&http.Cookie{Name: "access_token", Value: testJWTFor(t, userID, role)})
	w := httptest.NewRecorder()
	authHandler(db).ServeHTTP(w, r)
	return w
}

func mustDecode(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response (%s): %v", w.Body.String(), err)
	}
}

func TestListMine_AdminAlwaysGetsAllModules(t *testing.T) {
	db := setupTestDB(t)
	resetTables(t, db)

	w := doAs(t, db, 1, "admin", http.MethodGet, "/mine", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin list mine: %d %s", w.Code, w.Body.String())
	}
	var resp minePermissionsResponse
	mustDecode(t, w, &resp)
	if len(resp.Modules) != len(AllModules) {
		t.Fatalf("admin modules = %v, want all of %v", resp.Modules, AllModules)
	}
}

func TestListMine_GuestDefaultsToNoAccess(t *testing.T) {
	db := setupTestDB(t)
	resetTables(t, db)

	w := doAs(t, db, 2, "guest", http.MethodGet, "/mine", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("guest list mine: %d %s", w.Code, w.Body.String())
	}
	var resp minePermissionsResponse
	mustDecode(t, w, &resp)
	if len(resp.Modules) != 0 {
		t.Fatalf("fresh guest modules = %v, want none (deny by default)", resp.Modules)
	}
}

func TestUpdateGuest_GrantsReflectInListMine(t *testing.T) {
	db := setupTestDB(t)
	resetTables(t, db)

	w := doAs(t, db, 1, "admin", http.MethodPut, "/guest", map[string]any{
		"modules": map[string]bool{"gym": true, "finances": true},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("admin update guest: %d %s", w.Code, w.Body.String())
	}

	w = doAs(t, db, 2, "guest", http.MethodGet, "/mine", nil)
	var mine minePermissionsResponse
	mustDecode(t, w, &mine)
	got := map[string]bool{}
	for _, m := range mine.Modules {
		got[m] = true
	}
	if !got["gym"] || !got["finances"] || got["dashboard"] || got["ytdlp"] {
		t.Fatalf("guest modules after grant = %v, want exactly gym+finances", mine.Modules)
	}

	// Toggling one back off doesn't touch the other.
	w = doAs(t, db, 1, "admin", http.MethodPut, "/guest", map[string]any{
		"modules": map[string]bool{"gym": false},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("admin revoke gym: %d %s", w.Code, w.Body.String())
	}
	w = doAs(t, db, 2, "guest", http.MethodGet, "/mine", nil)
	mustDecode(t, w, &mine)
	got = map[string]bool{}
	for _, m := range mine.Modules {
		got[m] = true
	}
	if got["gym"] || !got["finances"] {
		t.Fatalf("guest modules after revoking gym = %v, want only finances", mine.Modules)
	}
}

func TestUpdateGuest_RejectsUnknownModule(t *testing.T) {
	db := setupTestDB(t)
	resetTables(t, db)

	w := doAs(t, db, 1, "admin", http.MethodPut, "/guest", map[string]any{
		"modules": map[string]bool{"db-manager": true},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("admin grant db-manager: status = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
	w = doAs(t, db, 1, "admin", http.MethodPut, "/guest", map[string]any{
		"modules": map[string]bool{"configuration": true},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("admin grant configuration: status = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
}

func TestGuestEndpoints_RejectNonAdmin(t *testing.T) {
	db := setupTestDB(t)
	resetTables(t, db)

	w := doAs(t, db, 2, "guest", http.MethodGet, "/guest", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest list guest permissions: status = %d, want 403 (body %s)", w.Code, w.Body.String())
	}
	w = doAs(t, db, 2, "guest", http.MethodPut, "/guest", map[string]any{"modules": map[string]bool{"gym": true}})
	if w.Code != http.StatusForbidden {
		t.Fatalf("guest update guest permissions: status = %d, want 403 (body %s)", w.Code, w.Body.String())
	}
}

func TestListGuest_ReturnsAllModulesEvenUnset(t *testing.T) {
	db := setupTestDB(t)
	resetTables(t, db)

	w := doAs(t, db, 1, "admin", http.MethodGet, "/guest", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin list guest: %d %s", w.Code, w.Body.String())
	}
	var resp listGuestPermissionsResponse
	mustDecode(t, w, &resp)
	if len(resp.Modules) != len(AllModules) {
		t.Fatalf("guest permission rows = %v, want one per AllModules entry (%v)", resp.Modules, AllModules)
	}
	for _, m := range resp.Modules {
		if m.Enabled {
			t.Fatalf("fresh guest should default every module to disabled, got %+v", m)
		}
	}
}
