package migrate

import "testing"

// TestQuoteIdentifierEscapesEmbeddedQuoteChar proves that a malicious
// identifier containing the dialect's own quote character cannot break out
// of the quoted identifier and inject arbitrary SQL: the character must be
// doubled (escaped), not passed through verbatim.
func TestQuoteIdentifierEscapesEmbeddedQuoteChar(t *testing.T) {
	pg := &PostgresDialect{}
	if got, want := pg.quoteIdentifier(`evil" ; DROP TABLE users; --`), `"evil"" ; DROP TABLE users; --"`; got != want {
		t.Errorf("PostgresDialect.quoteIdentifier: got %q, want %q", got, want)
	}

	sq := &SQLiteDialect{}
	if got, want := sq.quoteIdentifier(`evil" ; DROP TABLE users; --`), `"evil"" ; DROP TABLE users; --"`; got != want {
		t.Errorf("SQLiteDialect.quoteIdentifier: got %q, want %q", got, want)
	}

	my := &MySQLDialect{}
	if got, want := my.quoteIdentifier("evil` ; DROP TABLE users; --"), "`evil`` ; DROP TABLE users; --`"; got != want {
		t.Errorf("MySQLDialect.quoteIdentifier: got %q, want %q", got, want)
	}
}

// TestCreateTableSQLRejectsMaliciousIdentifier proves that identifiers
// containing quote/backtick characters (or otherwise failing
// ValidateIdentifier's rules) are rejected outright at the SQL-generation
// entry point, rather than silently accepted and merely escaped.
func TestCreateTableSQLRejectsMaliciousIdentifier(t *testing.T) {
	maliciousTable := CreateTable{
		Name: `users"; DROP TABLE secrets; --`,
		AddFields: []AddField{
			{Name: "id", Type: "integer"},
		},
	}

	dialects := map[string]Dialect{
		"postgres": &PostgresDialect{},
		"mysql":    &MySQLDialect{},
		"sqlite":   &SQLiteDialect{},
	}

	for name, d := range dialects {
		_, err := d.CreateTableSQL(maliciousTable, true)
		if err == nil {
			t.Errorf("%s: CreateTableSQL did not reject malicious table name", name)
		}
	}

	maliciousColumn := CreateTable{
		Name: "users",
		AddFields: []AddField{
			{Name: `id"; DROP TABLE secrets; --`, Type: "integer"},
		},
	}
	for name, d := range dialects {
		_, err := d.CreateTableSQL(maliciousColumn, true)
		if err == nil {
			t.Errorf("%s: CreateTableSQL did not reject malicious column name", name)
		}
	}
}

// TestValidateSQLIdentifier exercises the helper directly.
func TestValidateSQLIdentifier(t *testing.T) {
	if err := ValidateSQLIdentifier("field", "valid_name"); err != nil {
		t.Errorf("expected valid identifier to pass, got error: %v", err)
	}
	if err := ValidateSQLIdentifier("field", `bad"name`); err == nil {
		t.Error("expected identifier with embedded quote to be rejected")
	}
	if err := ValidateSQLIdentifier("field", "bad`name"); err == nil {
		t.Error("expected identifier with embedded backtick to be rejected")
	}
	// Reserved SQL keywords are valid identifiers here: every identifier
	// ValidateSQLIdentifier guards is always emitted through a dialect's
	// quoteIdentifier, so keyword status alone isn't a rejection reason.
	if err := ValidateSQLIdentifier("field", "action"); err != nil {
		t.Errorf("expected reserved-keyword identifier 'action' to pass, got error: %v", err)
	}
}

// TestIsReservedKeyword exercises the informational reserved-keyword helper,
// which is intentionally NOT used by ValidateIdentifier/ValidateSQLIdentifier
// (see their doc comments) but is kept available for callers that need to
// know whether an identifier requires quoting in a context that doesn't
// already quote it.
func TestIsReservedKeyword(t *testing.T) {
	for _, word := range []string{"select", "TABLE", "Action", "order", "user"} {
		if !IsReservedKeyword(word) {
			t.Errorf("expected %q to be recognized as a reserved keyword", word)
		}
	}
	for _, word := range []string{"users", "email_address", "created_at"} {
		if IsReservedKeyword(word) {
			t.Errorf("expected %q to NOT be recognized as a reserved keyword", word)
		}
	}
}
