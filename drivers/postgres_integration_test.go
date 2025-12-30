package drivers

import (
	"os"
	"testing"
)

// These are integration tests and will be skipped unless TEST_POSTGRES_DSN is
// provided in the environment. They exercise dollar-quoted functions, semicolons
// in strings, and transactional rollback behavior.
func TestPostgresApplySQL_WithDollarQuotedFunctionAndRollback(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("skipping postgres integration test; set TEST_POSTGRES_DSN to run")
	}
	p, err := NewPostgresDriver(dsn)
	if err != nil {
		t.Fatalf("failed to create postgres driver: %v", err)
	}
	defer func() { _ = p.DB().Close() }()

	// Ensure clean state
	_, _ = p.DB().Exec("DROP TABLE IF EXISTS tx_postgres_test;")
	_, _ = p.DB().Exec("DROP FUNCTION IF EXISTS test_fn_increment(integer);")

	// 1) Test rollback on failure (typo INSRT)
	bad := `CREATE FUNCTION test_fn_increment(i integer) RETURNS integer AS $$
BEGIN
  RETURN i + 1;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE tx_postgres_test (id serial PRIMARY KEY, v integer);
INSERT INTO tx_postgres_test (v) VALUES (1);

INSRT INTO tx_postgres_test (v) VALUES (2);`

	err = p.ApplySQL([]string{bad})
	if err == nil {
		t.Fatalf("expected error from malformed SQL but got nil")
	}
	// Verify nothing committed: no table
	var count int
	err = p.DB().QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_name='tx_postgres_test'").Scan(&count)
	if err != nil {
		t.Fatalf("failed check for table existence: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no table 'tx_postgres_test' after rollback, found %d", count)
	}

	// 2) Successful multi-statement should commit
	succ := `CREATE FUNCTION test_fn_increment(i integer) RETURNS integer AS $$
BEGIN
  RETURN i + 1;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE tx_postgres_test (id serial PRIMARY KEY, v integer);
INSERT INTO tx_postgres_test (v) VALUES (5);`
	if err := p.ApplySQL([]string{succ}); err != nil {
		t.Fatalf("expected success applying SQL, got %v", err)
	}
	// verify function exists and row is present
	err = p.DB().QueryRow("SELECT count(*) FROM pg_proc WHERE proname='test_fn_increment'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query pg_proc: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected function 'test_fn_increment' to exist after successful migration")
	}
	err = p.DB().QueryRow("SELECT count(*) FROM tx_postgres_test").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query tx_postgres_test: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row in tx_postgres_test after commit, got %d", count)
	}

	// cleanup
	_, _ = p.DB().Exec("DROP TABLE IF EXISTS tx_postgres_test;")
	_, _ = p.DB().Exec("DROP FUNCTION IF EXISTS test_fn_increment(integer);")
}
