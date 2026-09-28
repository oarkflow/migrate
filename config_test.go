package migrate

import (
	"strings"
	"testing"
)

func TestRedactDSNPostgres(t *testing.T) {
	dsn := "host=localhost port=5432 user=admin dbname=mydb password=Sup3rSecret! sslmode=disable"
	got := redactDSN(dsn)
	if got == dsn {
		t.Fatalf("expected redacted DSN to differ from original")
	}
	if strings.Contains(got, "Sup3rSecret!") {
		t.Errorf("redacted DSN still contains the password: %s", got)
	}
	if !strings.Contains(got, "password="+redactedPlaceholder) {
		t.Errorf("expected redacted DSN to contain placeholder, got: %s", got)
	}
	if !strings.Contains(got, "sslmode=disable") {
		t.Errorf("expected unrelated DSN fields to be preserved, got: %s", got)
	}
}

func TestRedactDSNMySQL(t *testing.T) {
	dsn := "admin:Sup3rSecret!@tcp(localhost:3306)/mydb?charset=utf8mb4"
	got := redactDSN(dsn)
	if strings.Contains(got, "Sup3rSecret!") {
		t.Errorf("redacted DSN still contains the password: %s", got)
	}
	if !strings.Contains(got, "admin:"+redactedPlaceholder+"@") {
		t.Errorf("expected redacted DSN to mask the mysql password, got: %s", got)
	}
	if !strings.Contains(got, "tcp(localhost:3306)/mydb?charset=utf8mb4") {
		t.Errorf("expected unrelated DSN fields to be preserved, got: %s", got)
	}
}

func TestRedactDSNSchemeStyle(t *testing.T) {
	dsn := "postgres://admin:Sup3rSecret!@localhost:5432/mydb?sslmode=disable"
	got := redactDSN(dsn)
	if strings.Contains(got, "Sup3rSecret!") {
		t.Errorf("redacted DSN still contains the password: %s", got)
	}
	if !strings.Contains(got, "://admin:"+redactedPlaceholder+"@") {
		t.Errorf("expected redacted DSN to mask the scheme-style password, got: %s", got)
	}
}

func TestRedactDSNNoPasswordUnchanged(t *testing.T) {
	dsn := "file:test.db?cache=shared"
	if got := redactDSN(dsn); got != dsn {
		t.Errorf("expected DSN without a password to be unchanged, got: %s", got)
	}
	if got := redactDSN(""); got != "" {
		t.Errorf("expected empty DSN to remain empty, got: %s", got)
	}
}

func TestGetDSNStillContainsRealPasswordForConnecting(t *testing.T) {
	// redactDSN must only affect what is logged; the real DSN used to connect
	// still needs the actual password.
	cfg := &MigrateConfig{
		Database: DatabaseConfig{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "admin",
			Password: "Sup3rSecret!",
			Database: "mydb",
		},
	}
	dsn := cfg.GetDSN()
	if !strings.Contains(dsn, "Sup3rSecret!") {
		t.Fatalf("expected GetDSN to retain the real password, got: %s", dsn)
	}
	if strings.Contains(redactDSN(dsn), "Sup3rSecret!") {
		t.Fatalf("expected redactDSN to strip the password from the logged copy")
	}
}
