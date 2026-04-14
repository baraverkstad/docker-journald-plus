package driver

import (
	"regexp"
	"strings"
)

// defaultTimestampPatterns lists timestamp patterns to strip (without ^ anchor).
// Order matters: more specific patterns should come first.
var defaultTimestampPatterns = []string{
	// Apache error log: Wed Oct 15 19:41:46.123456 2019
	`[A-Z][a-z]{2} [A-Z][a-z]{2}\s{1,2}\d{1,2} \d{2}:\d{2}:\d{2}(\.\d{1,6})? \d{4}`,

	// ISO 8601 and common variants:
	//   2024-01-15T10:30:45.123456789Z
	//   2024-01-15T10:30:45.123+02:00
	//   2024-01-15 10:30:45,123 UTC
	//   2024-01-15 10:30:45 +0000
	//   2024-01-15 10:30:45
	//   2024-01-15  6:36:00  (double space or single-digit hour)
	// Covers: Log4j2, Logback, Python, Ruby, MySQL 5.7+, PostgreSQL, Docker
	// Note: timezone abbreviations limited to Z/UTC/GMT to avoid matching
	// log level words like ERROR, WARN, INFO, DEBUG.
	`\[?\d{4}-\d{2}-\d{2}[T ][\d ]?\d:\d{2}:\d{2}([.,]\d{1,9})?(\s?(Z|[+-]\d{2}:?\d{2}))?(\s+(UTC|GMT))?\]?`,

	// Go log / nginx error: 2024/01/15 10:30:45 or 2024/01/15 10:30:45.000000
	`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(\.\d{1,6})?`,

	// Apache/nginx CLF: 15/Oct/2024:10:30:45 +0200 (optionally bracketed)
	`\[?\d{2}/[A-Z][a-z]{2}/\d{4}:\d{2}:\d{2}:\d{2}\s*[+-]\d{4}\]?`,

	// Log4j DATE format: 14 Nov 2017 20:30:20,434
	`\d{1,2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2}([.,]\d{1,3})?`,

	// Syslog: Jan 15 10:30:45 or Jan  5 10:30:45
	`[A-Z][a-z]{2}\s{1,2}\d{1,2} \d{2}:\d{2}:\d{2}`,
}

// buildTimestampPattern combines defaultTimestampPatterns into a single
// anchored alternation regex: ^(pat1|pat2|...).
func buildTimestampPattern() string {
	return "^(" + strings.Join(defaultTimestampPatterns, "|") + ")"
}

// trailingSep matches common separators after a timestamp.
var trailingSep = regexp.MustCompile(`^[\s:|\-]*`)

// StripTimestamp removes a leading timestamp from a log line.
// Returns the stripped line, or the original if no pattern matches.
func StripTimestamp(line []byte, re *regexp.Regexp) []byte {
	loc := re.FindIndex(line)
	if loc == nil {
		return line
	}
	rest := line[loc[1]:]
	if sepLoc := trailingSep.FindIndex(rest); sepLoc != nil && sepLoc[1] > 0 {
		rest = rest[sepLoc[1]:]
	}
	return rest
}
