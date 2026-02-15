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
	PriorityPrefix        bool
	PriorityDefaultStdout Priority
	PriorityDefaultStderr Priority
	PriorityMatchers      []priorityMatcher // ordered emerg..debug

	// JSON parsing
	ParseJSON       bool
	JSONLevelKeys   []string // Keys to check for level/severity
	JSONMessageKeys []string // Keys to check for message body

	// Field extraction
	FieldExtractors []fieldExtractor // Regex patterns to extract custom fields
}

type priorityMatcher struct {
	Priority Priority
	Regex    *regexp.Regexp
}

type fieldExtractor struct {
	FieldName string
	Regex     *regexp.Regexp
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

	"parse-json":        true,
	"json-level-keys":   true,
	"json-message-keys": true,
}

// ParseConfig validates and parses a map of log-opt key/value pairs.
func ParseConfig(opts map[string]string) (*Config, error) {
	for key := range opts {
		if !knownOpts[key] && !strings.HasPrefix(key, "field-") {
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
	// Each pattern allows up to 30 chars prefix to handle cases like:
	//   "2026-02-15 15:15:16 0 [Note] InnoDB:..." after timestamp stripping -> " 0 [Note] InnoDB:..."
	matchKeys := []struct {
		opt        string
		pri        Priority
		defaultPat string // default regex if option not set; empty = no default
	}{
		{"priority-match-emerg", PriEmerg, ""},
		{"priority-match-alert", PriAlert, ""},
		{"priority-match-crit", PriCrit, `^.{0,30}(CRITICAL|\[Critical\])`},
		{"priority-match-err", PriErr, `^.{0,30}(ERROR|FATAL|\[ERROR\]|\[Fatal\])`},
		{"priority-match-warning", PriWarning, `^.{0,30}(WARN|WARNING|\[Warning\])`},
		{"priority-match-notice", PriNotice, `^.{0,30}\[Note\]`},
		{"priority-match-info", PriInfo, ""},
		{"priority-match-debug", PriDebug, `^.{0,30}(DEBUG|\[Debug\])`},
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

	// Parse JSON options
	if v, ok := opts["parse-json"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid parse-json %q: must be true or false", v)
		}
		cfg.ParseJSON = b
	}

	// JSON level keys (comma-separated, defaults to "level,severity,log_level")
	if v, ok := opts["json-level-keys"]; ok && v != "" {
		cfg.JSONLevelKeys = strings.Split(v, ",")
		for i := range cfg.JSONLevelKeys {
			cfg.JSONLevelKeys[i] = strings.TrimSpace(cfg.JSONLevelKeys[i])
		}
	} else {
		cfg.JSONLevelKeys = []string{"level", "severity", "log_level"}
	}

	// JSON message keys (comma-separated, defaults to "message,msg,log")
	if v, ok := opts["json-message-keys"]; ok && v != "" {
		cfg.JSONMessageKeys = strings.Split(v, ",")
		for i := range cfg.JSONMessageKeys {
			cfg.JSONMessageKeys[i] = strings.TrimSpace(cfg.JSONMessageKeys[i])
		}
	} else {
		cfg.JSONMessageKeys = []string{"message", "msg", "log"}
	}

	// Field extractors (field-FIELDNAME options)
	for key, pattern := range opts {
		if !strings.HasPrefix(key, "field-") {
			continue
		}
		fieldName := strings.TrimPrefix(key, "field-")
		if fieldName == "" {
			return nil, fmt.Errorf("invalid field extractor key %q: field name cannot be empty", key)
		}
		if pattern == "" {
			return nil, fmt.Errorf("invalid field extractor %q: pattern cannot be empty", key)
		}
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid field extractor %q pattern %q: %w", key, pattern, err)
		}
		// Validate that the pattern has at least one capture group
		if r.NumSubexp() == 0 {
			return nil, fmt.Errorf("invalid field extractor %q pattern %q: must contain at least one capture group ()", key, pattern)
		}
		cfg.FieldExtractors = append(cfg.FieldExtractors, fieldExtractor{
			FieldName: fieldName,
			Regex:     r,
		})
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

// ExtractFields applies field extractors to a message and returns extracted field values.
// Returns a map of field names to extracted values.
func (c *Config) ExtractFields(message string) map[string]string {
	if len(c.FieldExtractors) == 0 {
		return nil
	}
	fields := make(map[string]string)
	for _, extractor := range c.FieldExtractors {
		matches := extractor.Regex.FindStringSubmatch(message)
		if len(matches) > 1 {
			// Use first capture group
			fields[extractor.FieldName] = matches[1]
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}
