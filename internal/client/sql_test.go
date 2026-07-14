package client

import (
	"context"
	"testing"
)

func TestSQLClientExecOnLocalDuckDB(t *testing.T) {
	c := NewSQLClient("") // empty token -> plain local duckdb for the unit test
	if err := c.Exec(context.Background(), "CREATE TABLE t(x INT);"); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func TestSQLClientQueryRoundTrip(t *testing.T) {
	c := NewSQLClient("")
	rows, closeDB, err := c.Query(context.Background(), "SELECT 42 AS x;")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer closeDB()
	defer rows.Close()

	if !rows.Next() {
		t.Fatalf("expected one row, got none")
	}
	var x int
	if err := rows.Scan(&x); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if x != 42 {
		t.Fatalf("expected 42, got %d", x)
	}
	if rows.Next() {
		t.Fatalf("expected exactly one row")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
}

func TestDSNForEmptyTokenIsLocal(t *testing.T) {
	if got := dsnFor(""); got != "" {
		t.Fatalf("empty token: want %q, got %q", "", got)
	}
}

func TestDSNForTokenBuildsMDDSN(t *testing.T) {
	if got := dsnFor("abc"); got != "md:?motherduck_token=abc&saas_mode=true" {
		t.Fatalf("token dsn mismatch: got %q", got)
	}
}

func TestDSNForTokenURLEncodesToken(t *testing.T) {
	if got := dsnFor("a b/c+d"); got != "md:?motherduck_token=a+b%2Fc%2Bd&saas_mode=true" {
		t.Fatalf("token not url-encoded: got %q", got)
	}
}
