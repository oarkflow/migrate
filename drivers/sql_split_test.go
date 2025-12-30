//go:build ignore

package drivers

import (
	"strings"
	"testing"
)

func TestSplitDollarQuotedFunction(t *testing.T) {
	sql := `
CREATE FUNCTION example() RETURNS void AS $$
BEGIN
  PERFORM 1;
END;
$$ LANGUAGE plpgsql;

COMMENT ON TABLE stage_transitions IS 'Defines conditional routing between stages';
`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmts[0])), "CREATE FUNCTION") {
		t.Fatalf("first statement should be function creation, got: %s", stmts[0])
	}
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmts[1])), "COMMENT ON TABLE") {
		t.Fatalf("second statement should be comment, got: %s", stmts[1])
	}
}

func TestSplitRespectsSingleQuotesAndComments(t *testing.T) {
	sql := "INSERT INTO t (c) VALUES ('value;with;semis'); -- comment; with semis\nSELECT 1;"
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
}
