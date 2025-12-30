package drivers

import "strings"

// splitSQLStatements splits SQL into statements while respecting quoted strings,
// dollar-quoted string tags (e.g., $$ ... $$ or $tag$ ... $tag$), and SQL
// comments ("--", "#", and block comments "/* ... */"). It returns trimmed
// non-empty statements without the trailing semicolons.
func splitSQLStatements(query string) []string {
	var stmts []string
	s := query
	l := len(s)
	start := 0
	inSingle := false
	inDouble := false
	inLineComment := false
	inBlockComment := false
	var dollarTag string
	for i := 0; i < l; i++ {
		ch := s[i]
		// handle end of line comment
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		// handle end of block comment
		if inBlockComment {
			if ch == '*' && i+1 < l && s[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		// handle dollar quote
		if dollarTag != "" {
			if len(dollarTag) <= l-i && s[i:i+len(dollarTag)] == dollarTag {
				i += len(dollarTag) - 1
				dollarTag = ""
			}
			continue
		}
		// handle single- and double-quotes
		if inSingle {
			if ch == '\'' {
				// SQL escapes single-quote by doubling it: ''
				if i+1 < l && s[i+1] == '\'' {
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		// not inside any quoting/comment
		if ch == '-' && i+1 < l && s[i+1] == '-' {
			inLineComment = true
			i++
			continue
		}
		if ch == '#' {
			inLineComment = true
			continue
		}
		if ch == '/' && i+1 < l && s[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}
		if ch == '\'' {
			inSingle = true
			continue
		}
		if ch == '"' {
			inDouble = true
			continue
		}
		if ch == '$' {
			// Try to parse a dollar tag like $tag$ or $$
			j := i + 1
			for j < l && isDollarTagChar(s[j]) {
				j++
			}
			if j < l && s[j] == '$' {
				dollarTag = s[i : j+1]
				i = j
				continue
			}
			// otherwise treat as regular char
			continue
		}
		if ch == ';' {
			stmt := strings.TrimSpace(s[start:i])
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
			start = i + 1
			continue
		}
	}
	if start < l {
		tail := strings.TrimSpace(s[start:])
		if tail != "" {
			stmts = append(stmts, tail)
		}
	}
	return stmts
}

func isDollarTagChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}
