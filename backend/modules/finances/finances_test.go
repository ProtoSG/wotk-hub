package finances

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

// Test harness for the finances module.
//
// These are integration tests against a real Postgres instance (the
// truncate-first dev convention guarantees clean data). They exercise
// genuine SQL semantics that no in-process fake can reproduce: CHECK
// constraints, FOR UPDATE locking, and the CASE-based cardBalance
// computation. Set FINANCES_TEST_DB to a DSN, or the default points at
// the local workhub_test cluster. Tests skip when the DB is unreachable.

const testJWTSecret = "finances-test-secret"

func testDSN() string {
	if v := os.Getenv("FINANCES_TEST_DB"); v != "" {
		return v
	}
	return "postgres://workhub:workhub@localhost:5432/workhub_test?sslmode=disable"
}

// setupTestDB returns a *sql.DB with the full schema migrated. It skips
// the test if the Postgres instance is unreachable — these are integration
// tests and silently passing without a DB would give false confidence.
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

// resetFinanceTables gives each subtest a clean ledger: every finance row
// is truncated RESTART IDENTITY, CASCADE handles FK ordering between
// transactions↔cards and savings_contributions↔goals↔cards. A single
// admin user (id=1) is seeded because created_by REFERENCES users(id) —
// the JWT subject binds as created_by on every insert.
//
// categories isn't in the TRUNCATE list on purpose — it's shared reference
// data, not per-test state — but budgets/subscriptions/categories now all
// have created_by REFERENCES users(id) too (per-user scoping), so
// TRUNCATE users ... CASCADE truncates categories right along with it
// (CASCADE truncates the whole dependent table, not just matching rows).
// Re-running the idempotent Migrate (its category seed is
// ON CONFLICT DO NOTHING) re-seeds it every time, same as a fresh DB.
func resetFinanceTables(t *testing.T, db *sql.DB) {
	t.Helper()
	const q = `TRUNCATE TABLE
		users, cards, transactions, subscriptions, budgets,
		savings_goals, savings_contributions, couple_dates, refresh_tokens
		RESTART IDENTITY CASCADE`
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO users (id, email, password_hash, name, role)
		 VALUES (1, 'admin@test', 'x', 'Admin', 'admin')`); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("re-seed after truncate: %v", err)
	}
}

// testUserJWT returns a JWT cookie value for user 1 / admin signed with the
// test secret the router's JWTAuth uses. created_by on inserts is the JWT
// subject, which must exist as a users row (see resetFinanceTables).
func testUserJWT(t *testing.T) string {
	t.Helper()
	return testJWTFor(t, 1, "admin")
}

// testJWTFor signs a cookie value for an arbitrary user/role — used by the
// per-user ownership tests, which need a second identity beyond the seeded
// admin (see seedGuestUser).
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

// seedGuestUser inserts a second user (id=2, role=guest) — resetFinanceTables
// only seeds the admin (id=1), so ownership-boundary tests that need two
// distinct identities call this after resetFinanceTables.
func seedGuestUser(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO users (id, email, password_hash, name, role)
		 VALUES (2, 'guest@test', 'x', 'Guest', 'guest')`); err != nil {
		t.Fatalf("seed guest: %v", err)
	}
}

// doAs is do (see below) with an explicit caller identity instead of the
// default admin cookie — for exercising scopeToOwner's guest-vs-admin split.
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

// authHandler wraps the finances routes behind JWTAuth so handler tests
// exercise the real auth→context→scopeToOwner pipeline using a cookie.
func authHandler(db *sql.DB) http.Handler {
	return middleware.JWTAuth(testJWTSecret)(Routes(db, testJWTSecret))
}

// do performs an authenticated JSON request against the test router and
// returns the recorded response. body may be nil.
func do(t *testing.T, db *sql.DB, method, path string, body any) *httptest.ResponseRecorder {
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
	r.AddCookie(&http.Cookie{Name: "access_token", Value: testUserJWT(t)})
	w := httptest.NewRecorder()
	authHandler(db).ServeHTTP(w, r)
	return w
}

// testCreditLimitCents is the limit given to fixture cards that stand in for a
// credit card. Any value above zero would do — the production code only asks
// whether the limit is positive.
const testCreditLimitCents = 1_000_000

// insertCard is the minimal fixture: a card row owned by the seeded admin.
//
// kind is "debito" | "credito" | "prepago". Cards no longer carry a type
// column — credit behaviour is inferred from credit_limit_cents > 0 (see
// cardBalance in transactions.go) — so the argument survives only as the
// fixture's name and as the switch for that limit. Tests that need a starting
// balance seed it themselves with a transfer.
func insertCard(t *testing.T, db *sql.DB, kind string) int64 {
	t.Helper()
	creditLimit := int64(0)
	if kind == "credito" {
		creditLimit = testCreditLimitCents
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO cards (name, bank, credit_limit_cents, created_by)
		 VALUES ($1, 'test', $2, 1) RETURNING id`,
		"card-"+kind, creditLimit,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert card (%s): %v", kind, err)
	}
	return id
}

// seedCardBalance inserts a transfer into the card so its computed balance
// starts at amount (mirrors CreateCard's seed transfer, but from a test
// fixture so we control the exact number).
func seedCardBalance(t *testing.T, db *sql.DB, cardID int64, amount int64) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO transactions (type, amount_cents, category, description, occurred_on, created_by, to_card_id)
		 VALUES ('transfer', $1, 'transferencia', 'seed', CURRENT_DATE, 1, $2)`,
		amount, cardID,
	); err != nil {
		t.Fatalf("seed card balance: %v", err)
	}
}

// cardBalanceJSON fetches one card's computed BalanceCents via the live
// list endpoint and the read-side cardsBaseQuery CASE — the same SQL that
// cardBalance uses, exposed to clients as BalanceCents.
func cardBalanceJSON(t *testing.T, db *sql.DB, cardID int64) int64 {
	t.Helper()
	w := do(t, db, http.MethodGet, "/cards", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /cards: %d %s", w.Code, w.Body.String())
	}
	var resp listCardsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode cards: %v", err)
	}
	for _, c := range resp.Cards {
		if c.ID == cardID {
			return c.BalanceCents
		}
	}
	t.Fatalf("card %d not found in list", cardID)
	return 0
}

// cardStateJSON fetches one card's computed BalanceCents AND
// UsedCreditCents via the live list endpoint (cardsBaseQuery CASE — the
// mirrored read shape of cardBalance). Used by tests that need to assert
// both fields at once (e.g. the credito refund edge: balance AND used
// credit must stay unchanged).
func cardStateJSON(t *testing.T, db *sql.DB, cardID int64) (balance, usedCredit int64) {
	t.Helper()
	w := do(t, db, http.MethodGet, "/cards", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /cards: %d %s", w.Code, w.Body.String())
	}
	var resp listCardsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode cards: %v", err)
	}
	for _, c := range resp.Cards {
		if c.ID == cardID {
			return c.BalanceCents, c.UsedCreditCents
		}
	}
	t.Fatalf("card %d not found in list", cardID)
	return 0, 0
}

// mustDecode JSON-decodes a response body into dst, failing the test on
// error — a small shared helper for the ownership tests, which mostly just
// need to inspect a list/single response's shape.
func mustDecode(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response (%s): %v", w.Body.String(), err)
	}
}

// recordNoAuth runs a request with no access_token cookie at all, straight
// against Routes (not authHandler, which wraps everything — including the
// public GET /categories — in a hard-401-on-missing-cookie JWTAuth layer
// that would defeat the point of this helper). Only GET /categories is
// actually reachable unauthenticated; everything else lives behind
// RequireAuth inside Routes itself (see routes.go).
func recordNoAuth(db *sql.DB, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	Routes(db, testJWTSecret).ServeHTTP(w, r)
	return w
}

// doDirectAs is doAs, but straight against Routes like recordNoAuth — not
// authHandler, whose outer JWTAuth wraps every route including
// GET /categories and would populate UserFromContext even if the route's
// own OptionalAuth middleware silently didn't. main.go mounts finances at
// /api/finances with no such outer wrapper (see routes.go's own doc
// comment), so this is what a real logged-in request to /categories
// actually goes through — the case that hid the OptionalAuth bug when every
// other test here went through authHandler instead.
func doDirectAs(t *testing.T, db *sql.DB, userID int64, role, method, path string, body any) *httptest.ResponseRecorder {
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
	Routes(db, testJWTSecret).ServeHTTP(w, r)
	return w
}

// errBody extracts the APIError message from a response body (best effort).
func errBody(w *httptest.ResponseRecorder) string {
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	return e.Message
}
