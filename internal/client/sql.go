package client

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	_ "github.com/marcboeker/go-duckdb/v2" // md: connections go through the duckdb driver
)

// SQLClient runs data-plane DDL against MotherDuck as the OWNING account. The
// REST APIClient (admin token) cannot own catalogs or author shares; this is the
// separate connection those operations require. The account token is supplied
// per call site (per Terraform resource), not from provider config.
type SQLClient struct{ token string }

func NewSQLClient(token string) *SQLClient { return &SQLClient{token: token} }

// dsnFor builds the duckdb DSN for a token. Empty token -> a local in-memory
// duckdb (unit tests). saas_mode=true is required for MotherDuck data-plane
// DDL: it binds the session to the token's owning account so CREATE
// DATABASE/SHARE/ATTACH execute under that account's catalog ownership rather
// than being rejected as an admin/service operation.
func dsnFor(token string) string {
	if token == "" {
		return ""
	}
	q := url.Values{"motherduck_token": {token}, "saas_mode": {"true"}}
	return "md:?" + q.Encode()
}

func (c *SQLClient) open() (*sql.DB, error) {
	return sql.Open("duckdb", dsnFor(c.token))
}

func (c *SQLClient) Exec(ctx context.Context, stmt string) error {
	db, err := c.open()
	if err != nil {
		return fmt.Errorf("data-plane exec: %w", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("data-plane exec: %w", err)
	}
	return nil
}

// Query runs q and returns its rows along with a close function that releases
// the underlying *sql.DB (a live cgo duckdb/MotherDuck handle). Callers must
// invoke close when done with the rows, typically `defer close()`; closing the
// *sql.Rows alone only returns the connection to the pool and leaves the handle
// leaked. close is safe to call after the rows are consumed or closed.
func (c *SQLClient) Query(ctx context.Context, q string) (*sql.Rows, func() error, error) {
	db, err := c.open()
	if err != nil {
		return nil, nil, fmt.Errorf("data-plane query: %w", err)
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("data-plane query: %w", err)
	}
	return rows, db.Close, nil
}
