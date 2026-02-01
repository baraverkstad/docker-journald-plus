package driver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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

var priorityNames = map[string]Priority{
	"emerg":   PriEmerg,
	"alert":   PriAlert,
	"crit":    PriCrit,
	"err":     PriErr,
	"warning": PriWarning,
	"notice":  PriNotice,
	"info":    PriInfo,
	"debug":   PriDebug,
}

// Config holds parsed and validated configuration for a single container.
type Config struct {
	// Inherited journald options
	Tag         string
	Labels      []string
	LabelsRegex *regexp.Regexp
	Env         []string
	EnvRegex    *regexp.Regexp

	// Multiline
	MultilineRegex    *regexp.Regexp // nil = disabled
	MultilineTimeout  time.Duration
	MultilineMaxLines int
	MultilineMaxBytes int
	MultilineSep      string

	// Timestamp stripping
	StripTimestamp         bool
	StripTimestampPatterns []*regexp.Regexp // compiled; nil if disabled

	// Priority
	PriorityPrefix       bool
	PriorityDefaultStdout Priority
	PriorityDefaultStderr Priority
	PriorityMatchers      []priorityMatcher // ordered emerg..debug
}

type priorityMatcher struct {
	Priority Priority
	Regex    *regexp.Regexp
}

// known option keys
var knownOpts = map[string]bool{
	"tag":          true,
	"labels":       true,
	"labels-regex": true,
	"env":          true,
	"env-regex":    true,

	"multiline-regex":     true,
	"multiline-timeout":   true,
	"multiline-max-lines": true,
	"multiline-max-bytes": true,
	"multiline-separator": true,

	"priority-prefix":         true,
	"priority-default-stdout": true,
	"priority-default-stderr": true,
	"priority-match-emerg":    true,
	"priority-match-alert":    true,
	"priority-match-crit":     true,
	"priority-match-err":      true,
	"priority-match-warning":  true,
	"priority-match-notice":   true,
	"priority-match-info":     true,
	"priority-match-debug":    true,

	"strip-timestamp":       true,
	"strip-timestamp-regex": true,
}

// ParseConfig validates and parses a map of log-opt key/value pairs.
func ParseConfig(opts map[string]string) (*Config, error) {
	for key := range opts {
		if !knownOpts[key] {
			return nil, fmt.Errorf("unknown log-opt %q", key)
		}
	}

	cfg := &Config{
		MultilineTimeout:      10 * time.Millisecond,
		MultilineMaxLines:     100,
		MultilineMaxBytes:     1048576,
		MultilineSep:          "\n",
		PriorityPrefix:        true,
		PriorityDefaultStdout: PriInfo,
		PriorityDefaultStderr: PriErr,
	}

	// Tag
	cfg.Tag = opts["tag"]

	// Labels
	if v, ok := opts["labels"]; ok && v != "" {
		cfg.Labels = strings.Split(v, ",")
	}
	if v, ok := opts["labels-regex"]; ok && v != "" {
		r, err := regexp.Compile(v)
		if err != nil {
			return nil, fmt.Errorf("invalid labels-regex %q: %w", v, err)
		}
		cfg.LabelsRegex = r
	}

	// Env
	if v, ok := opts["env"]; ok && v != "" {
		cfg.Env = strings.Split(v, ",")
	}
	if v, ok := opts["env-regex"]; ok && v != "" {
		r, err := regexp.Compile(v)
		if err != nil {
			return nil, fmt.Errorf("invalid env-regex %q: %w", v, err)
		}
		cfg.EnvRegex = r
	}

	// Multiline regex
	if v, ok := opts["multiline-regex"]; ok {
		if v == "" {
			cfg.MultilineRegex = nil // explicitly disabled
		} else {
			r, err := regexp.Compile(v)
			if err != nil {
				return nil, fmt.Errorf("invalid multiline-regex %q: %w", v, err)
			}
			cfg.MultilineRegex = r
		}
	} else {
		// Default: lines starting with whitespace are continuations
		cfg.MultilineRegex = regexp.MustCompile(`^\s`)
	}

	// Multiline timeout
	if v, ok := opts["multiline-timeout"]; ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid multiline-timeout %q: %w", v, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("multiline-timeout must be positive, got %v", d)
		}
		cfg.MultilineTimeout = d
	}

	// Multiline max lines
	if v, ok := opts["multiline-max-lines"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid multiline-max-lines %q: must be a positive integer", v)
		}
		cfg.MultilineMaxLines = n
	}

	// Multiline max bytes
	if v, ok := opts["multiline-max-bytes"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid multiline-max-bytes %q: must be a positive integer", v)
		}
		cfg.MultilineMaxBytes = n
	}

	// Multiline separator
	if v, ok := opts["multiline-separator"]; ok {
		cfg.MultilineSep = v
	}

	// Priority prefix
	if v, ok := opts["priority-prefix"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid priority-prefix %q: must be true or false", v)
		}
		cfg.PriorityPrefix = b
	}

	// Priority defaults
	if v, ok := opts["priority-default-stdout"]; ok {
		p, err := parsePriorityName(v)
		if err != nil {
			return nil, fmt.Errorf("invalid priority-default-stdout: %w", err)
		}
		cfg.PriorityDefaultStdout = p
	}
	if v, ok := opts["priority-default-stderr"]; ok {
		p, err := parsePriorityName(v)
		if err != nil {
			return nil, fmt.Errorf("invalid priority-default-stderr: %w", err)
		}
		cfg.PriorityDefaultStderr = p
	}

	// Priority matchers (ordered emerg..debug)
	matchKeys := []struct {
		opt        string
		pri        Priority
		defaultPat string // default regex if option not set; empty = no default
	}{
		{"priority-match-emerg", PriEmerg, ""},
		{"priority-match-alert", PriAlert, ""},
		{"priority-match-crit", PriCrit, `^CRITICAL|^\[Critical\]`},
		{"priority-match-err", PriErr, `^ERROR|^FATAL|^\[ERROR\]|^\[Fatal\]`},
		{"priority-match-warning", PriWarning, `^WARN|^WARNING|^\[Warning\]`},
		{"priority-match-notice", PriNotice, `^\[Note\]`},
		{"priority-match-info", PriInfo, ""},
		{"priority-match-debug", PriDebug, `^DEBUG|^\[Debug\]`},
	}
	for _, mk := range matchKeys {
		pattern := mk.defaultPat
		if v, ok := opts[mk.opt]; ok {
			pattern = v // user override (empty string disables)
		}
		if pattern == "" {
			continue
		}
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid %s %q: %w", mk.opt, pattern, err)
		}
		cfg.PriorityMatchers = append(cfg.PriorityMatchers, priorityMatcher{
			Priority: mk.pri,
			Regex:    r,
		})
	}

	// Timestamp stripping
	if v, ok := opts["strip-timestamp"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid strip-timestamp %q: must be true or false", v)
		}
		cfg.StripTimestamp = b
	}
	if cfg.StripTimestamp {
		if v, ok := opts["strip-timestamp-regex"]; ok && v != "" {
			// User-provided single pattern
			patterns, err := compileTimestampPatterns([]string{v})
			if err != nil {
				return nil, fmt.Errorf("invalid strip-timestamp-regex %q: %w", v, err)
			}
			cfg.StripTimestampPatterns = patterns
		} else {
			// Use built-in patterns
			patterns, err := compileTimestampPatterns(defaultTimestampPatterns)
			if err != nil {
				return nil, fmt.Errorf("compiling default timestamp patterns: %w", err)
			}
			cfg.StripTimestampPatterns = patterns
		}
	}

	return cfg, nil
}

func parsePriorityName(s string) (Priority, error) {
	p, ok := priorityNames[strings.ToLower(s)]
	if !ok {
		return 0, fmt.Errorf("unknown priority %q (valid: emerg, alert, crit, err, warning, notice, info, debug)", s)
	}
	return p, nil
}
