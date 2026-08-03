package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
	"workhub/store"

	"github.com/golang-jwt/jwt/v5"

	_ "github.com/lib/pq"
)

// Test harness for RequireAuth (cookie-JWT-first, Bearer-API-key-fallback).
// Integration tests against a real Postgres instance, following the same
// conventions as finances_test.go: skip if unreachable, truncate before
// each test, seed fixtures with direct SQL.

const testJWTSecret = "middleware-test-secret"

func testDSN() string {
	if v := os.Getenv("MIDDLEWARE_TEST_DB"); v != "" {
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
		t.Skipf("test postgres unreachable (%v); set MIDDLEWARE_TEST_DB to run integration tests", err)
	}
	if err := store.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// resetAuthTables truncates users, api_keys and refresh_tokens (CASCADE
// also clears any other FK-referencing rows), then callers seed exactly the
// fixtures each test needs via insertTestUser/insertTestAPIKey.
func resetAuthTables(t *testing.T, db *sql.DB) {
	t.Helper()
	const q = `TRUNCATE TABLE users, api_keys, refresh_tokens RESTART IDENTITY CASCADE`
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func insertTestUser(t *testing.T, db *sql.DB, email, role string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(
		`INSERT INTO users (email, password_hash, name, role) VALUES ($1, 'x', 'Test User', $2) RETURNING id`,
		email, role,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	return id
}

// hashRawKey mirrors auth.hashToken/newAPIKey's SHA-256 hex hashing so
// fixtures inserted here match what RequireAuth computes from the raw
// Bearer value.
func hashRawKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func insertTestAPIKey(t *testing.T, db *sql.DB, userID int64, rawKey string, revoked bool) {
	t.Helper()
	hash := hashRawKey(rawKey)
	if revoked {
		_, err := db.Exec(
			`INSERT INTO api_keys (user_id, name, key_hash, revoked_at) VALUES ($1, 'test key', $2, now())`,
			userID, hash,
		)
		if err != nil {
			t.Fatalf("insert revoked api key: %v", err)
		}
		return
	}
	_, err := db.Exec(
		`INSERT INTO api_keys (user_id, name, key_hash) VALUES ($1, 'test key', $2)`,
		userID, hash,
	)
	if err != nil {
		t.Fatalf("insert api key: %v", err)
	}
}

func testCookieJWT(t *testing.T, userID int64, role string) string {
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

// recordingHandler is the "next" handler RequireAuth wraps in these tests —
// it echoes the context values UserFromContext exposes back as response
// headers so tests can assert on them.
func recordingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, role, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "missing user in context", http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-User-ID", strconv.FormatInt(userID, 10))
		w.Header().Set("X-Role", role)
		w.WriteHeader(http.StatusOK)
	}
}

func TestRequireAuth_APIKey(t *testing.T) {
	db := setupTestDB(t)
	resetAuthTables(t, db)

	userID := insertTestUser(t, db, "keyuser@test", "admin")
	insertTestAPIKey(t, db, userID, "wh_validkey", false)
	insertTestAPIKey(t, db, userID, "wh_revokedkey", true)

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"valid unrevoked key", "Bearer wh_validkey", http.StatusOK},
		{"revoked key", "Bearer wh_revokedkey", http.StatusUnauthorized},
		{"unknown key", "Bearer wh_doesnotexist", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()

			RequireAuth(db, testJWTSecret)(recordingHandler()).ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				if got := w.Header().Get("X-User-ID"); got != strconv.FormatInt(userID, 10) {
					t.Errorf("X-User-ID = %q, want %q", got, strconv.FormatInt(userID, 10))
				}
				if got := w.Header().Get("X-Role"); got != "admin" {
					t.Errorf("X-Role = %q, want %q", got, "admin")
				}
			}
		})
	}
}

func TestRequireAuth_MalformedAuthorizationHeader(t *testing.T) {
	db := setupTestDB(t)
	resetAuthTables(t, db)

	tests := []struct {
		name      string
		setHeader bool
		header    string
	}{
		{"no Bearer prefix", true, "Basic sometoken"},
		{"empty bearer value", true, "Bearer "},
		{"header absent", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.setHeader {
				r.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()

			RequireAuth(db, testJWTSecret)(recordingHandler()).ServeHTTP(w, r)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body %s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestRequireAuth_CookieJWTStillWorks(t *testing.T) {
	db := setupTestDB(t)
	resetAuthTables(t, db)

	userID := insertTestUser(t, db, "cookieuser@test", "guest")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "access_token", Value: testCookieJWT(t, userID, "guest")})
	w := httptest.NewRecorder()

	RequireAuth(db, testJWTSecret)(recordingHandler()).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-User-ID"); got != strconv.FormatInt(userID, 10) {
		t.Errorf("X-User-ID = %q, want %q", got, strconv.FormatInt(userID, 10))
	}
	if got := w.Header().Get("X-Role"); got != "guest" {
		t.Errorf("X-Role = %q, want %q", got, "guest")
	}
}

func TestRequireAuth_NoCredentials(t *testing.T) {
	db := setupTestDB(t)
	resetAuthTables(t, db)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	RequireAuth(db, testJWTSecret)(recordingHandler()).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %s)", w.Code, w.Body.String())
	}
}
