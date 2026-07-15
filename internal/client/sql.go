package client

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"

	"github.com/marcboeker/go-duckdb/v2" // md: reached as an ATTACHed catalog, not the main database
)

// SQLClient runs data-plane DDL against MotherDuck as the OWNING account. The
// REST APIClient (admin token) cannot own catalogs or author shares; this is the
// separate connection those operations require. The account token is supplied
// per call site (per Terraform resource), not from provider config.
type SQLClient struct{ token string }

func NewSQLClient(token string) *SQLClient { return &SQLClient{token: token} }

// dbCache holds one pooled *sql.DB per non-empty token for the process lifetime.
// A token's configuration is identical across resources, so its connections are
// interchangeable; sharing the pool amortizes the one-time instance bootstrap
// (see newBootstrappedDB) across the many DDL statements a parallel apply issues
// under that token. Each entry is initialized under its own sync.Once so a slow
// first bootstrap for one token never blocks cache hits for another. The empty
// token (unit tests) is never cached, keeping each test on its own throwaway
// in-memory database.
type cachedDB struct {
	once sync.Once
	db   *sql.DB
	err  error
}

var (
	dbCacheMu sync.Mutex
	dbCache   = map[string]*cachedDB{}
)

// bootstrap is the instance-global statement sequence that reaches MotherDuck as
// an ATTACHed catalog under the account token. An empty token yields no
// statements: the instance is then a bare in-memory duckdb (unit tests). Each
// statement configures the DuckDB *instance* once — SET motherduck_token and
// ATTACH 'md:' both error if replayed on an already-initialized instance — so
// this runs exactly once per instance in runBootstrap, never per connection.
func (c *SQLClient) bootstrap() []string {
	if c.token == "" {
		return nil
	}
	return []string{
		"INSTALL motherduck",
		"LOAD motherduck",
		"SET motherduck_token='" + c.token + "'",
		"ATTACH 'md:'",
	}
}

// newBootstrappedDB opens a DB whose MAIN database is a fresh in-memory duckdb
// and, for a real token, initializes MotherDuck as an ATTACHed catalog once on
// the instance. MotherDuck is deliberately never the main database: DuckDB caches
// a main database by path and refuses to reopen it under a different
// configuration for the process lifetime, and the account token is part of that
// configuration — opening `md:` directly admits only one token per process, so a
// second owner account or region fails with "same database file with a different
// configuration". Reaching MotherDuck through ATTACH moves the token into
// instance session state, which distinct tokens hold in distinct instances, so
// parallel applies over many tokens no longer collide.
//
// The connector runs no per-connection init: SET motherduck_token / ATTACH 'md:'
// are instance-global and error if replayed, so a shared connector (one instance,
// many pooled connections) must bootstrap exactly once. Connections opened later
// inherit the instance's attached catalog and token without re-running anything.
func (c *SQLClient) newBootstrappedDB() (*sql.DB, error) {
	connector, err := duckdb.NewConnector("", func(driver.ExecerContext) error { return nil })
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	if err := runBootstrap(db, c.bootstrap()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// runBootstrap executes the instance-global setup on a single connection, then
// returns it to the pool. A background context is used so a cancelled caller
// cannot leave the shared instance half-initialized.
func runBootstrap(db *sql.DB, stmts []string) error {
	if len(stmts) == 0 {
		return nil
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	for _, stmt := range stmts {
		if _, err := conn.ExecContext(context.Background(), stmt); err != nil {
			return err
		}
	}
	return nil
}

// acquire returns the *sql.DB to run against and a release function. For a real
// token the DB is the shared, cached pool and release is a no-op (the pool
// outlives the call). For the empty token it is a fresh throwaway DB that release
// closes, isolating unit tests from one another.
func (c *SQLClient) acquire() (*sql.DB, func(), error) {
	if c.token == "" {
		db, err := c.newBootstrappedDB()
		if err != nil {
			return nil, nil, err
		}
		return db, func() { _ = db.Close() }, nil
	}

	dbCacheMu.Lock()
	entry := dbCache[c.token]
	if entry == nil {
		entry = &cachedDB{}
		dbCache[c.token] = entry
	}
	dbCacheMu.Unlock()

	entry.once.Do(func() {
		entry.db, entry.err = c.newBootstrappedDB()
		if entry.err != nil {
			// Drop the failed entry so a later call retries instead of caching the error.
			dbCacheMu.Lock()
			delete(dbCache, c.token)
			dbCacheMu.Unlock()
		}
	})
	if entry.err != nil {
		return nil, nil, entry.err
	}
	return entry.db, func() {}, nil
}

func (c *SQLClient) Exec(ctx context.Context, stmt string) error {
	db, release, err := c.acquire()
	if err != nil {
		return fmt.Errorf("data-plane exec: %w", err)
	}
	defer release()
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("data-plane exec: %w", err)
	}
	return nil
}

// Query runs q and returns its rows along with a close function the caller must
// invoke when done (typically `defer close()`). close returns the borrowed
// connection to the shared pool by closing the rows; for the uncached empty-token
// DB it also closes that throwaway DB. The cached pool itself is never closed —
// it lives for the process so later calls under the same token reuse it.
func (c *SQLClient) Query(ctx context.Context, q string) (*sql.Rows, func() error, error) {
	db, release, err := c.acquire()
	if err != nil {
		return nil, nil, fmt.Errorf("data-plane query: %w", err)
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		release()
		return nil, nil, fmt.Errorf("data-plane query: %w", err)
	}
	return rows, func() error {
		rowsErr := rows.Close()
		release()
		return rowsErr
	}, nil
}
