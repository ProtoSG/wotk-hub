package gym

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

// Test harness for the gym module — same shape as finances' (see
// finances/finances_test.go), since gym now shares its per-user ownership
// model. Unlike finances, Routes(db) here has no internal auth split (see
// routes.go's doc comment: "authentication is applied by the caller in
// main.go"), so the harness wraps it in JWTAuth itself.

const testJWTSecret = "gym-test-secret"

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

// resetGymTables gives each subtest a clean slate. exercises isn't in the
// TRUNCATE list — it's a shared catalog seeded at Migrate time, not per-test
// state — but it now has created_by REFERENCES users(id) too, so
// TRUNCATE users ... CASCADE wipes it right along with everything else
// (same gotcha finances hit — see its resetFinanceTables comment). Re-run
// SeedExercises after truncating to restore the bundled catalog.
func resetGymTables(t *testing.T, db *sql.DB) {
	t.Helper()
	const q = `TRUNCATE TABLE
		users, routines, routine_exercises, workout_sessions, session_exercises,
		exercise_sets, exercises
		RESTART IDENTITY CASCADE`
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO users (id, email, password_hash, name, role)
		 VALUES (1, 'admin@test', 'x', 'Admin', 'admin')`); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if err := SeedExercises(db); err != nil {
		t.Fatalf("re-seed exercises after truncate: %v", err)
	}
}

func seedGuestUser(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO users (id, email, password_hash, name, role)
		 VALUES (2, 'guest@test', 'x', 'Guest', 'guest')`); err != nil {
		t.Fatalf("seed guest: %v", err)
	}
}

func testJWTFor(t *testing.T, userID int64, role string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  strconv.FormatInt(userID, 10),
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
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
