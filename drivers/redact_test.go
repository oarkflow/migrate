package drivers

import (
	"fmt"
	"strings"
	"testing"
)

func TestRedactDSN_PostgresStyle(t *testing.T) {
	dsn := "host=localhost port=5432 user=admin dbname=mydb password=Sup3rSecret! sslmode=disable"
	got := redactDSN(dsn)
	if strings.Contains(got, "Sup3rSecret!") {
		t.Fatalf("expected password to be redacted, got: %s", got)
	}
	want := "host=localhost port=5432 user=admin dbname=mydb password=***REDACTED*** sslmode=disable"
	if got != want {
		t.Fatalf("unexpected redaction result:\n got:  %s\n want: %s", got, want)
	}
}

func TestRedactDSN_MySQLStyle(t *testing.T) {
	dsn := "admin:Sup3rSecret!@tcp(localhost:3306)/mydb?charset=utf8mb4"
	got := redactDSN(dsn)
	if strings.Contains(got, "Sup3rSecret!") {
		t.Fatalf("expected password to be redacted, got: %s", got)
	}
	want := "admin:***@tcp(localhost:3306)/mydb?charset=utf8mb4"
	if got != want {
		t.Fatalf("unexpected redaction result:\n got:  %s\n want: %s", got, want)
	}
}

func TestRedactDSN_URIStyle(t *testing.T) {
	dsn := "postgres://admin:Sup3rSecret!@dbhost:5432/mydb"
	got := redactDSN(dsn)
	if strings.Contains(got, "Sup3rSecret!") {
		t.Fatalf("expected password to be redacted, got: %s", got)
	}
	want := "postgres://admin:***@dbhost:5432/mydb"
	if got != want {
		t.Fatalf("unexpected redaction result:\n got:  %s\n want: %s", got, want)
	}
}

func TestRedactDSN_EmbeddedInWrappedError(t *testing.T) {
	dsn := "host=localhost port=5432 user=admin dbname=mydb password=Sup3rSecret! sslmode=disable"
	// Simulate a driver error that echoes the DSN back verbatim, as some
	// underlying SQL drivers do.
	underlying := fmt.Errorf("dial failed for dsn %q: connection refused", dsn)
	wrapped := fmt.Errorf("failed to open connection: %s", redactDSN(underlying.Error()))
	if strings.Contains(wrapped.Error(), "Sup3rSecret!") {
		t.Fatalf("expected password to be redacted from wrapped error, got: %s", wrapped.Error())
	}
}

func TestRedactDSN_NoPassword(t *testing.T) {
	dsn := "file:./migrations.db"
	got := redactDSN(dsn)
	if got != dsn {
		t.Fatalf("expected dsn without credentials to be unchanged, got: %s", got)
	}
}
