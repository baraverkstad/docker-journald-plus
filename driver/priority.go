package driver

import (
	"fmt"
	"regexp"
	"strings"
)

// Priority represents a syslog/journal priority level.
type Priority int

const (
	PriEmerg   Priority = 0
	PriAlert   Priority = 1
	PriCrit    Priority = 2
	PriErr     Priority = 3
	PriWarning Priority = 4
	PriNotice  Priority = 5
	PriInfo    Priority = 6
	PriDebug   Priority = 7
)

var priorityPrefixPattern = regexp.MustCompile(`^<([0-7])>`)

var priorityNames = map[string]Priority{
	"emerg":       PriEmerg,
	"emergency":   PriEmerg,
	"alert":       PriAlert,
	"panic":       PriAlert,
	"crit":        PriCrit,
	"critical":    PriCrit,
	"fatal":       PriCrit,
	"err":         PriErr,
	"error":       PriErr,
	"warning":     PriWarning,
	"warn":        PriWarning,
	"notice":      PriNotice,
	"info":        PriInfo,
	"information": PriInfo,
	"debug":       PriDebug,
	"trace":       PriDebug,
}

// priorityPatterns maps each priority level to its built-in match pattern.
// Patterns allow up to 30 chars prefix to handle cases like:
//
//	"2026-02-15 15:15:16 0 [Note] InnoDB:..." after timestamp stripping -> "0 [Note] InnoDB:..."
var priorityPatterns = map[Priority]string{
	PriEmerg:   "",
	PriAlert:   "",
	PriCrit:    `^.{0,30}(CRITICAL|\[Critical\])`,
	PriErr:     `^.{0,30}(ERROR|FATAL|\[ERROR\]|\[Fatal\])`,
	PriWarning: `^.{0,30}(WARN|WARNING|\[Warning\])`,
	PriNotice:  "",
	PriInfo:    "",
	PriDebug:   `^.{0,30}(DEBUG|\[Debug\])`,
}

type priorityMatcher struct {
	Priority Priority
	Regex    *regexp.Regexp
}

func parsePriorityName(s string) (Priority, error) {
	p, ok := priorityNames[strings.ToLower(s)]
	if !ok {
		return 0, fmt.Errorf("unknown priority %q", s)
	}
	return p, nil
}

// DetectPriority determines the journal priority for a message and returns
// the (possibly stripped) message. Checks sd-daemon prefix, then regex
// patterns, then falls back to the configured default for the source.
func DetectPriority(cfg *Config, firstLine []byte, source string) (Priority, []byte) {
	if cfg.PriorityPrefix {
		if loc := priorityPrefixPattern.FindSubmatchIndex(firstLine); loc != nil {
			n := firstLine[loc[2]] - '0'
			stripped := firstLine[loc[1]:]
			return Priority(n), stripped
		}
	}

	for _, m := range cfg.PriorityMatchers {
		if m.Regex.Match(firstLine) {
			return m.Priority, firstLine
		}
	}

	if source == "stderr" {
		return cfg.PriorityDefaultStderr, firstLine
	}
	return cfg.PriorityDefaultStdout, firstLine
}
