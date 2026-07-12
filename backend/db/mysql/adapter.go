package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
	"workhub/db"

	"github.com/go-sql-driver/mysql"
)

type Adapter struct{}

// idleTTL is how long a cached connection may sit unused before the
// background sweep closes it.
const idleTTL = 5 * time.Minute

// maxRows caps how many rows RunQuery will scan into memory for a single
// SELECT, to avoid unbounded memory use on huge result sets.
const maxRows = 1000

// pingTimeout bounds how long a cache-hit health check (or the initial
// connect) may take, so one unreachable/slow target DB can't hang a caller
// (or, transitively, anyone else waiting on the same per-key init lock)
// indefinitely.
const pingTimeout = 5 * time.Second

type cacheEntry struct {
	conn     *sql.DB
	lastUsed time.Time
	active   int
}

var (
	// cacheMu guards only the cache map itself (reads/writes of entries),
	// never a network call. Dialing/pinging happen outside this lock so a
	// slow/unreachable target DB can't block lookups for unrelated keys.
	cacheMu sync.Mutex
	cache   = map[string]*cacheEntry{}

	// initMu holds one lock per cache key, created lazily, so concurrent
	// first-time callers for the SAME key serialize on dial/reconnect
	// while callers for DIFFERENT keys never block on each other.
	initMuGuard sync.Mutex
	initMu      = map[string]*sync.Mutex{}
)

// keyInitLock returns (creating if necessary) the per-key mutex used to
// serialize connect/reconnect attempts for key.
func keyInitLock(key string) *sync.Mutex {
	initMuGuard.Lock()
	defer initMuGuard.Unlock()
	m, ok := initMu[key]
	if !ok {
		m = &sync.Mutex{}
		initMu[key] = m
	}
	return m
}

func init() {
	go sweepLoop()
}

// sweepLoop periodically closes and evicts connections that have been idle
// longer than idleTTL. It runs for the lifetime of the process; there's no
// shutdown hook for it since these are ad-hoc connections to user-supplied
// external databases, not the app's own store.
func sweepLoop() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		now := time.Now()
		cacheMu.Lock()
		for key, e := range cache {
			if e.active == 0 && now.Sub(e.lastUsed) > idleTTL {
				e.conn.Close()
				delete(cache, key)

				initMuGuard.Lock()
				delete(initMu, key)
				initMuGuard.Unlock()
			}
		}
		cacheMu.Unlock()
	}
}

// beginActivity marks conn's cache entry as busy so the idle sweeper won't
// close it mid-query, and refreshes lastUsed. endActivity must be called
// (typically via defer) once the query/exec finishes.
func beginActivity(key string) {
	cacheMu.Lock()
	if e, ok := cache[key]; ok {
		e.active++
		e.lastUsed = time.Now()
	}
	cacheMu.Unlock()
}

func endActivity(key string) {
	cacheMu.Lock()
	if e, ok := cache[key]; ok {
		e.active--
		e.lastUsed = time.Now()
	}
	cacheMu.Unlock()
}

func cacheKey(cfg db.ConnectionConfig) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s", cfg.Dialect, cfg.Host, cfg.Port, cfg.User, cfg.Database, cfg.Password)
}

// open returns a cached, pooled *sql.DB for cfg, reusing an existing
// connection when one is cached and still healthy instead of opening a new
// TCP + auth handshake on every DB Manager call. Callers must not Close the
// returned *sql.DB — it's shared and lives until the idle sweep evicts it.
func open(cfg db.ConnectionConfig) (*sql.DB, error) {
	key := cacheKey(cfg)

	// Fast path: briefly lock only to read the cache entry, then ping
	// outside the lock. cacheMu must never be held across a network call,
	// or one slow/unreachable target DB would serialize every other
	// connection lookup behind it.
	if conn := lookupHealthy(key); conn != nil {
		return conn, nil
	}

	// Slow path: serialize connect/reconnect attempts for THIS key only,
	// so two concurrent first-time (or first-broken) callers for the same
	// key can't both dial and have one pool silently overwritten (and
	// leaked, never Closed). Callers for other keys are unaffected since
	// each key has its own lock.
	lock := keyInitLock(key)
	lock.Lock()
	defer lock.Unlock()

	// Re-check: another goroutine may have already (re)dialed this key
	// while we were waiting for the per-key lock.
	if conn := lookupHealthy(key); conn != nil {
		return conn, nil
	}

	// Drop whatever stale/broken entry is left, if any.
	cacheMu.Lock()
	if e, ok := cache[key]; ok {
		e.conn.Close()
		delete(cache, key)
	}
	cacheMu.Unlock()

	dsn := buildDSN(cfg)
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxIdleTime(idleTTL)

	cacheMu.Lock()
	cache[key] = &cacheEntry{conn: conn, lastUsed: time.Now()}
	cacheMu.Unlock()

	return conn, nil
}

// lookupHealthy returns the cached connection for key if one exists and
// responds to a bounded PingContext, or nil otherwise. The cache map is
// only locked long enough to copy the entry pointer; the ping itself runs
// unlocked so it can never block unrelated keys.
func lookupHealthy(key string) *sql.DB {
	cacheMu.Lock()
	e, ok := cache[key]
	if ok {
		e.lastUsed = time.Now()
	}
	cacheMu.Unlock()

	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := e.conn.PingContext(ctx); err != nil {
		return nil
	}
	return e.conn
}

// buildDSN builds the MySQL DSN via mysql.Config.FormatDSN instead of naive
// string concatenation, so a user/password/database containing special
// characters (spaces, quotes, @, :, ?) is escaped correctly. tls=preferred
// makes the driver attempt TLS and fall back to plaintext if the server
// doesn't support it (go-sql-driver/mysql supports this value natively, no
// custom tls.Config registration needed).
func buildDSN(cfg db.ConnectionConfig) string {
	c := mysql.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		DBName:               cfg.Database,
		Timeout:              5 * time.Second,
		ParseTime:            true,
		AllowNativePasswords: true,
		TLSConfig:            "preferred",
	}
	return c.FormatDSN()
}

func (Adapter) TestConnection(cfg db.ConnectionConfig) error {
	conn, err := open(cfg)
	if err != nil {
		return err
	}
	key := cacheKey(cfg)
	beginActivity(key)
	defer endActivity(key)
	return conn.Ping()
}

func (Adapter) ListTables(cfg db.ConnectionConfig) ([]string, error) {
	conn, err := open(cfg)
	if err != nil {
		return nil, err
	}
	key := cacheKey(cfg)
	beginActivity(key)
	defer endActivity(key)

	rows, err := conn.Query(
		`SELECT TABLE_NAME FROM information_schema.TABLES
		 WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME`,
		cfg.Database,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (Adapter) GetSchema(cfg db.ConnectionConfig, table string) ([]db.ColumnInfo, error) {
	conn, err := open(cfg)
	if err != nil {
		return nil, err
	}
	key := cacheKey(cfg)
	beginActivity(key)
	defer endActivity(key)

	pks, err := primaryKeyColumns(conn, cfg.Database, table)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(
		`SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT
		 FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		 ORDER BY ORDINAL_POSITION`,
		cfg.Database, table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []db.ColumnInfo
	for rows.Next() {
		var c db.ColumnInfo
		var nullable string
		var def sql.NullString
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &def); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		if def.Valid {
			c.Default = &def.String
		}
		c.IsPrimaryKey = pks[c.Name]
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// primaryKeyColumns returns the set of column names that are part of
// table's primary key, determined from real catalog metadata rather than a
// name-based guess.
func primaryKeyColumns(conn *sql.DB, schema, table string) (map[string]bool, error) {
	rows, err := conn.Query(
		`SELECT COLUMN_NAME
		 FROM information_schema.KEY_COLUMN_USAGE
		 WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'`,
		schema, table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pks := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		pks[name] = true
	}
	return pks, rows.Err()
}

// returnsRows reports whether query is expected to produce a result set
// (SELECT/WITH/SHOW/EXPLAIN/PRAGMA, or any statement with a RETURNING
// clause) as opposed to a plain DML statement (INSERT/UPDATE/DELETE)
// that should go through Exec so its RowsAffected is reported correctly.
func returnsRows(query string) bool {
	q := strings.ToUpper(strings.TrimSpace(query))
	for _, prefix := range []string{"SELECT", "WITH", "SHOW", "EXPLAIN", "PRAGMA"} {
		if strings.HasPrefix(q, prefix) {
			return true
		}
	}
	return strings.Contains(q, "RETURNING")
}

func (Adapter) RunQuery(cfg db.ConnectionConfig, query string) (db.QueryResult, error) {
	conn, err := open(cfg)
	if err != nil {
		return db.QueryResult{}, err
	}
	key := cacheKey(cfg)
	beginActivity(key)
	defer endActivity(key)

	start := time.Now()

	if !returnsRows(query) {
		res, err := conn.Exec(query)
		if err != nil {
			return db.QueryResult{}, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return db.QueryResult{}, err
		}
		return db.QueryResult{
			Columns:         []string{},
			Rows:            []map[string]any{},
			RowCount:        int(affected),
			ExecutionTimeMs: time.Since(start).Milliseconds(),
		}, nil
	}

	rows, err := conn.Query(query)
	if err != nil {
		return db.QueryResult{}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return db.QueryResult{}, err
	}

	var result []map[string]any
	truncated := false
	for rows.Next() {
		if len(result) >= maxRows {
			truncated = true
			break
		}
		vals := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return db.QueryResult{}, err
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			if b, ok := vals[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = vals[i]
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return db.QueryResult{}, err
	}

	if result == nil {
		result = []map[string]any{}
	}

	return db.QueryResult{
		Columns:         columns,
		Rows:            result,
		RowCount:        len(result),
		ExecutionTimeMs: time.Since(start).Milliseconds(),
		Truncated:       truncated,
	}, nil
}

func (Adapter) GetForeignKeys(cfg db.ConnectionConfig) ([]db.ForeignKey, error) {
	conn, err := open(cfg)
	if err != nil {
		return nil, err
	}
	key := cacheKey(cfg)
	beginActivity(key)
	defer endActivity(key)

	rows, err := conn.Query(
		`SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
		 FROM information_schema.KEY_COLUMN_USAGE
		 WHERE REFERENCED_TABLE_NAME IS NOT NULL AND TABLE_SCHEMA = ?`,
		cfg.Database,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []db.ForeignKey
	for rows.Next() {
		var fk db.ForeignKey
		if err := rows.Scan(&fk.FromTable, &fk.FromColumn, &fk.ToTable, &fk.ToColumn); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	if fks == nil {
		fks = []db.ForeignKey{}
	}
	return fks, rows.Err()
}
