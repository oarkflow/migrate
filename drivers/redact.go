package drivers

import "regexp"

// redactPasswordParam matches key=value style password fields, e.g. the
// "password=xxxx" segment produced by MigrateConfig.GetDSN() for Postgres DSNs.
var redactPasswordParam = regexp.MustCompile(`(?i)(password=)[^\s]+`)

// redactUserInfo matches "user:password@" style credentials embedded in a DSN
// or connection URI, e.g. the MySQL DSN form "user:pass@tcp(host:port)/db" or
// a generic "scheme://user:pass@host" URI.
var redactUserInfo = regexp.MustCompile(`([^\s/:@]+):([^\s/@]+)@`)

// redactDSN returns a copy of s with any embedded database password redacted.
// It is safe to call on DSNs, connection strings, and arbitrary error/log
// messages that may have a DSN embedded within them (some drivers include the
// DSN verbatim in connection errors), since it only rewrites recognizable
// password segments and leaves everything else untouched.
func redactDSN(s string) string {
	if s == "" {
		return s
	}
	s = redactPasswordParam.ReplaceAllString(s, `${1}***REDACTED***`)
	s = redactUserInfo.ReplaceAllString(s, `${1}:***@`)
	return s
}
