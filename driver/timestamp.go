package driver

import "regexp"

// defaultTimestampPatterns defines the built-in timestamp patterns to strip.
// Each pattern matches a timestamp at the start of a log line.
// Order matters: more specific patterns should come first.
var defaultTimestampPatterns = []string{
	// Apache error log: Wed Oct 15 19:41:46.123456 2019
	`^[A-Z][a-z]{2} [A-Z][a-z]{2}\s{1,2}\d{1,2} \d{2}:\d{2}:\d{2}(\.\d{1,6})? \d{4}`,

	// ISO 8601 and common variants:
	//   2024-01-15T10:30:45.123456789Z
	//   2024-01-15T10:30:45.123+02:00
	//   2024-01-15 10:30:45,123 UTC
	//   2024-01-15 10:30:45
	// Covers: Log4j2, Logback, Python, Ruby, MySQL 5.7+, PostgreSQL, Docker
	// Note: timezone abbreviations limited to Z/UTC/GMT to avoid matching
	// log level words like ERROR, WARN, INFO, DEBUG.
	`^\[?\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}([.,]\d{1,9})?(Z|[+-]\d{2}:?\d{2})?(\s+(UTC|GMT))?\]?`,

	// Go log / nginx error: 2024/01/15 10:30:45 or 2024/01/15 10:30:45.000000
	`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(\.\d{1,6})?`,

	// Apache/nginx CLF: 15/Oct/2024:10:30:45 +0200 (optionally bracketed)
	`^\[?\d{2}/[A-Z][a-z]{2}/\d{4}:\d{2}:\d{2}:\d{2}\s*[+-]\d{4}\]?`,

	// Log4j DATE format: 14 Nov 2017 20:30:20,434
	`^\d{1,2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2}([.,]\d{1,3})?`,

	// Syslog: Jan 15 10:30:45 or Jan  5 10:30:45
	`^[A-Z][a-z]{2}\s{1,2}\d{1,2} \d{2}:\d{2}:\d{2}`,
}

// trailingSep matches common separators after a timestamp.
var trailingSep = regexp.MustCompile(`^[\s:|\-]*`)

// compileTimestampPatterns compiles a list of regex pattern strings.
func compileTimestampPatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, r)
	}
	return compiled, nil
}

// StripTimestamp removes a leading timestamp from a log line using the
// provided compiled patterns. Returns the stripped line, or the original
// if no pattern matches.
func StripTimestamp(line []byte, patterns []*regexp.Regexp) []byte {
	for _, re := range patterns {
		loc := re.FindIndex(line)
		if loc == nil {
			continue
		}
		rest := line[loc[1]:]
		// Strip trailing separator (whitespace, dashes, pipes, colons)
		if sepLoc := trailingSep.FindIndex(rest); sepLoc != nil && sepLoc[1] > 0 {
			rest = rest[sepLoc[1]:]
		}
		// Only strip if there's something left
		if len(rest) > 0 {
			return rest
		}
		return line
	}
	return line
}
