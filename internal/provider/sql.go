package provider

import (
	"context"
	"database/sql"
	"strings"
)

// sqlQuerier is the read surface of client.SQLClient (satisfied by *SQLClient).
// Depending on this rather than the concrete client keeps read helpers narrow.
type sqlQuerier interface {
	Query(ctx context.Context, q string) (*sql.Rows, func() error, error)
}

// quoteIdent renders s as a double-quoted SQL identifier, escaping any embedded
// double quote by doubling it (DuckDB/ANSI rule). Every interpolated database
// name, share name, and grant username flows through this to stay injection-safe.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteLiteral renders s as a single-quoted SQL string literal, escaping any
// embedded single quote by doubling it (DuckDB/ANSI rule). Used for values
// compared as strings in WHERE clauses (e.g. the share-name filter on
// MD_INFORMATION_SCHEMA.OWNED_SHARES).
func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}
