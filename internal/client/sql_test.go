package client

import (
	"context"
	"os"
	"sync"
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

func TestBootstrapEmptyTokenIsLocal(t *testing.T) {
	if got := NewSQLClient("").bootstrap(); got != nil {
		t.Fatalf("empty token: want no bootstrap statements, got %q", got)
	}
}

func TestBootstrapTokenAttachesMotherDuck(t *testing.T) {
	got := NewSQLClient("abc").bootstrap()
	want := []string{
		"INSTALL motherduck",
		"LOAD motherduck",
		"SET motherduck_token='abc'",
		"ATTACH 'md:'",
	}
	if len(got) != len(want) {
		t.Fatalf("bootstrap length: got %d, want %d (%q)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bootstrap[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAccConcurrentSameToken guards against replaying instance-global setup per
// connection: SET motherduck_token and ATTACH 'md:' error on an already-attached
// instance, so a cached connector serving several concurrent connections under
// one token must bootstrap exactly once. Holding many rows open at once forces
// the pool past a single connection.
func TestAccConcurrentSameToken(t *testing.T) {
	tok := os.Getenv("MOTHERDUCK_TEST_TOKEN")
	if tok == "" {
		t.Skip("requires a live token: MOTHERDUCK_TEST_TOKEN")
	}
	ctx := context.Background()
	c := NewSQLClient(tok)

	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	closers := make([]func() error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, closeDB, err := c.Query(ctx, "SELECT count(*) FROM duckdb_databases() WHERE type = 'motherduck';")
			errs[i] = err
			closers[i] = closeDB
		}(i)
	}
	wg.Wait()
	for _, closeDB := range closers {
		if closeDB != nil {
			_ = closeDB()
		}
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent connection %d under one token failed: %v", i, err)
		}
	}
}

// TestAccConcurrentDistinctTokens is the regression guard for the process-wide
// `md:` main-database collision: opening MotherDuck as the main database admits
// only one token per process, so concurrent DDL under two owner tokens fails with
// "same database file with a different configuration". It runs only when a second
// distinct token is provided; a single token cannot reproduce the collision (a
// differing configuration is required). Set MOTHERDUCK_TEST_TOKEN_2 to a token for
// a different account (or a second PAT) to exercise it.
func TestAccConcurrentDistinctTokens(t *testing.T) {
	t1 := os.Getenv("MOTHERDUCK_TEST_TOKEN")
	t2 := os.Getenv("MOTHERDUCK_TEST_TOKEN_2")
	if t1 == "" || t2 == "" || t1 == t2 {
		t.Skip("requires two distinct live tokens: MOTHERDUCK_TEST_TOKEN and MOTHERDUCK_TEST_TOKEN_2")
	}
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, tok := range []string{t1, t2} {
		wg.Add(1)
		go func(i int, tok string) {
			defer wg.Done()
			_, closeDB, err := NewSQLClient(tok).Query(ctx, "SELECT 1;")
			if err == nil {
				_ = closeDB()
			}
			errs[i] = err
		}(i, tok)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent client %d failed under distinct tokens: %v", i, err)
		}
	}
}
