package driver

import "regexp"

var sdDaemonPrefix = regexp.MustCompile(`^<([0-7])>`)

// DetectPriority determines the journal priority for a message and returns
// the (possibly stripped) message. It checks in order:
// 1. sd-daemon <N> prefix (if enabled)
// 2. priority-match-* regex patterns (first match wins)
// 3. default based on source (stdout/stderr)
func DetectPriority(cfg *Config, firstLine []byte, source string) (Priority, []byte) {
	// 1. sd-daemon prefix
	if cfg.PriorityPrefix {
		if loc := sdDaemonPrefix.FindSubmatchIndex(firstLine); loc != nil {
			n := firstLine[loc[2]] - '0'
			stripped := firstLine[loc[1]:]
			return Priority(n), stripped
		}
	}

	// 2. Regex pattern matching
	for _, m := range cfg.PriorityMatchers {
		if m.Regex.Match(firstLine) {
			return m.Priority, firstLine
		}
	}

	// 3. Default based on source
	if source == "stderr" {
		return cfg.PriorityDefaultStderr, firstLine
	}
	return cfg.PriorityDefaultStdout, firstLine
}
