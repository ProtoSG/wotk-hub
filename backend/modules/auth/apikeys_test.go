package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"workhub/store"

	"github.com/golang-jwt/jwt/v5"
)

// Test harness for CreateAPIKey/ListAPIKeys/RevokeAPIKey. Integration tests
// against a real Postgres instance, following the same conventions as
// finances_test.go: skip if unreachable, truncate before each test, sign a
// JWT cookie directly rather than going through Login. The "postgres"
// driver is registered by handlers.go's github.com/lib/pq import, so no
// separate blank import is needed here.

const testSecret = "auth-test-secret"

func testDSN() string {
	if v := os.Getenv("AUTH_TEST_DB"); v != "" {
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
		t.Skipf("test postgres unreachable (%v); set AUTH_TEST_DB to run integration tests", err)
	}
	if err := store.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

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

func testUserJWT(t *testing.T, userID int64, role string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  strconv.FormatInt(userID, 10),
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return s
}

// authedRequest builds a request against the auth router carrying a valid
// session cookie for userID/role.
func authedRequest(t *testing.T, userID int64, role, method, path string, body any) *http.Request {
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
	r.AddCookie(&http.Cookie{Name: "access_token", Value: testUserJWT(t, userID, role)})
	return r
}

// createAPIKeyForUser mints a key for userID via the real handler (not a
// direct SQL insert), so the fixtures used by List/Revoke tests exercise
// the same code path CreateAPIKey's own test asserts on.
func createAPIKeyForUser(t *testing.T, db *sql.DB, userID int64, role, name string) apiKeyCreated {
	t.Helper()
	r := authedRequest(t, userID, role, http.MethodPost, "/keys", createAPIKeyRequest{Name: name})
	w := httptest.NewRecorder()
	Routes(db, testSecret, false).ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("create api key: status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp apiKeyCreated
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return resp
}

func TestCreateAPIKey(t *testing.T) {
	db := setupTestDB(t)
	resetAuthTables(t, db)
	userID := insertTestUser(t, db, "keyowner@test", "admin")

	resp := createAPIKeyForUser(t, db, userID, "admin", "My Key")

	if resp.Key == "" || !strings.HasPrefix(resp.Key, "wh_") {
		t.Fatalf("expected raw key with wh_ prefix, got %q", resp.Key)
	}
	if resp.Name != "My Key" {
		t.Errorf("Name = %q, want %q", resp.Name, "My Key")
	}

	var storedHash string
	if err := db.QueryRow(`SELECT key_hash FROM api_keys WHERE id = $1`, resp.ID).Scan(&storedHash); err != nil {
		t.Fatalf("query stored key: %v", err)
	}
	if wantHash := hashToken(resp.Key); storedHash != wantHash {
		t.Errorf("stored key_hash = %s, want sha256(raw) = %s", storedHash, wantHash)
	}
}

func TestListAPIKeys(t *testing.T) {
	db := setupTestDB(t)
	resetAuthTables(t, db)
	userA := insertTestUser(t, db, "usera@test", "admin")
	userB := insertTestUser(t, db, "userb@test", "guest")

	createAPIKeyForUser(t, db, userA, "admin", "keyA1")
	createAPIKeyForUser(t, db, userA, "admin", "keyA2")
	createAPIKeyForUser(t, db, userB, "guest", "keyB1")

	r := authedRequest(t, userA, "admin", http.MethodGet, "/keys", nil)
	w := httptest.NewRecorder()
	Routes(db, testSecret, false).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if body := w.Body.String(); strings.Contains(body, "key_hash") || strings.Contains(body, `"key"`) {
		t.Fatalf("response leaks raw key or hash: %s", body)
	}

	var keys []apiKeyView
	if err := json.Unmarshal(w.Body.Bytes(), &keys); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2 (only caller's own)", len(keys))
	}
	for _, k := range keys {
		if k.Name != "keyA1" && k.Name != "keyA2" {
			t.Errorf("unexpected key in userA's list: %+v", k)
		}
	}
}

func TestRevokeAPIKey(t *testing.T) {
	db := setupTestDB(t)
	resetAuthTables(t, db)
	userA := insertTestUser(t, db, "usera@test", "admin")
	userB := insertTestUser(t, db, "userb@test", "guest")

	keyA := createAPIKeyForUser(t, db, userA, "admin", "keyA")
	keyB := createAPIKeyForUser(t, db, userB, "guest", "keyB")

	t.Run("revoke own key", func(t *testing.T) {
		r := authedRequest(t, userA, "admin", http.MethodDelete, fmt.Sprintf("/keys/%d", keyA.ID), nil)
		w := httptest.NewRecorder()
		Routes(db, testSecret, false).ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
		}
		var revokedAt sql.NullString
		if err := db.QueryRow(`SELECT revoked_at FROM api_keys WHERE id = $1`, keyA.ID).Scan(&revokedAt); err != nil {
			t.Fatalf("query: %v", err)
		}
		if !revokedAt.Valid {
			t.Fatal("expected revoked_at to be set after revoke")
		}
	})

	t.Run("revoking an already-revoked key is 404", func(t *testing.T) {
		r := authedRequest(t, userA, "admin", http.MethodDelete, fmt.Sprintf("/keys/%d", keyA.ID), nil)
		w := httptest.NewRecorder()
		Routes(db, testSecret, false).ServeHTTP(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body %s)", w.Code, w.Body.String())
		}
	})

	t.Run("revoking another user's key is 404 and leaves it unrevoked", func(t *testing.T) {
		r := authedRequest(t, userA, "admin", http.MethodDelete, fmt.Sprintf("/keys/%d", keyB.ID), nil)
		w := httptest.NewRecorder()
		Routes(db, testSecret, false).ServeHTTP(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body %s)", w.Code, w.Body.String())
		}
		var revokedAt sql.NullString
		if err := db.QueryRow(`SELECT revoked_at FROM api_keys WHERE id = $1`, keyB.ID).Scan(&revokedAt); err != nil {
			t.Fatalf("query: %v", err)
		}
		if revokedAt.Valid {
			t.Fatal("userB's key must remain unrevoked when userA attempts to revoke it")
		}
	})
}
