package driver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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
	StripTimestamp      bool
	StripTimestampRegex *regexp.Regexp

	// Priority
	PriorityPrefix        bool
	PriorityDefaultStdout Priority
	PriorityDefaultStderr Priority
	PriorityMatchers      []priorityMatcher // ordered emerg..debug
	StripPriority         bool
	StripPriorityRegex    *regexp.Regexp // nil when strip-priority=false
	NormalizeWhitespace   bool

	// JSON parsing
	ParseJSON       bool
	JSONLevelKeys   []string // Keys to check for level/severity
	JSONMessageKeys []string // Keys to check for message body
	JSONSkipKeys    []string // Keys to ignore entirely
	JSONExtraInline bool     // false=journal fields (default), true=append remaining JSON to message

	// Field extraction
	FieldExtractors []fieldExtractor // Regex patterns to extract custom fields
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

	"strip-priority":       true,
	"strip-priority-regex": true,

	"normalize-whitespace": true,

	"json-parse":        true,
	"parse-json":        true, // deprecated alias for json-parse
	"json-level-keys":   true,
	"json-message-keys": true,
	"json-skip-keys":    true,
	"json-extra":        true,
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

	var err error

	cfg.Tag = opts["tag"]

	cfg.Labels = parseArrOpt(opts, "labels", nil)
	cfg.LabelsRegex, err = parseRegexOpt(opts, "labels-regex", "")
	if err != nil {
		return nil, err
	}

	cfg.Env = parseArrOpt(opts, "env", nil)
	cfg.EnvRegex, err = parseRegexOpt(opts, "env-regex", "")
	if err != nil {
		return nil, err
	}

	// Multiline (empty regex disables, absent uses default)
	cfg.MultilineRegex, err = parseRegexOpt(opts, "multiline-regex", `^\s`)
	if err != nil {
		return nil, err
	}
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
	cfg.MultilineMaxLines, err = parseIntOpt(opts, "multiline-max-lines", 100)
	if err != nil {
		return nil, err
	}
	cfg.MultilineMaxBytes, err = parseIntOpt(opts, "multiline-max-bytes", 1048576)
	if err != nil {
		return nil, err
	}
	if v, ok := opts["multiline-separator"]; ok {
		cfg.MultilineSep = v
	}

	// Priority
	cfg.PriorityPrefix, err = parseBoolOpt(opts, "priority-prefix", true)
	if err != nil {
		return nil, err
	}
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
	matchKeys := []struct {
		opt string
		pri Priority
	}{
		{"priority-match-emerg", PriEmerg},
		{"priority-match-alert", PriAlert},
		{"priority-match-crit", PriCrit},
		{"priority-match-err", PriErr},
		{"priority-match-warning", PriWarning},
		{"priority-match-notice", PriNotice},
		{"priority-match-info", PriInfo},
		{"priority-match-debug", PriDebug},
	}
	for _, mk := range matchKeys {
		r, err := parseRegexOpt(opts, mk.opt, priorityPatterns[mk.pri])
		if err != nil {
			return nil, err
		}
		if r != nil {
			cfg.PriorityMatchers = append(cfg.PriorityMatchers, priorityMatcher{
				Priority: mk.pri,
				Regex:    r,
			})
		}
	}

	// Timestamp stripping
	cfg.StripTimestamp, err = parseBoolOpt(opts, "strip-timestamp", false)
	if err != nil {
		return nil, err
	}
	if cfg.StripTimestamp {
		cfg.StripTimestampRegex, err = parseRegexOpt(opts, "strip-timestamp-regex", buildTimestampPattern())
		if err != nil {
			return nil, err
		}
	}

	// Priority stripping
	cfg.StripPriority, err = parseBoolOpt(opts, "strip-priority", false)
	if err != nil {
		return nil, err
	}
	if cfg.StripPriority {
		pattern := `(?i)^\[?(trace|debug|info|notice|note|warning|warn|critical|error|fatal|alert|emerg)\]?`
		cfg.StripPriorityRegex, err = parseRegexOpt(opts, "strip-priority-regex", pattern)
		if err != nil {
			return nil, err
		}
	}

	// Whitespace normalization
	cfg.NormalizeWhitespace, err = parseBoolOpt(opts, "normalize-whitespace", false)
	if err != nil {
		return nil, err
	}

	// JSON (json-parse is canonical, parse-json is the legacy alias)
	for _, key := range []string{"json-parse", "parse-json"} {
		if _, ok := opts[key]; ok {
			cfg.ParseJSON, err = parseBoolOpt(opts, key, false)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	cfg.JSONLevelKeys = parseArrOpt(opts, "json-level-keys", []string{"level", "severity", "log_level"})
	cfg.JSONMessageKeys = parseArrOpt(opts, "json-message-keys", []string{"message", "msg", "log"})
	cfg.JSONSkipKeys = parseArrOpt(opts, "json-skip-keys", nil)
	if v, ok := opts["json-extra"]; ok {
		switch v {
		case "fields", "":
			cfg.JSONExtraInline = false
		case "inline":
			cfg.JSONExtraInline = true
		default:
			return nil, fmt.Errorf("invalid json-extra %q: must be \"fields\" or \"inline\"", v)
		}
	}

	// Field extractors
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

func parseBoolOpt(opts map[string]string, key string, def bool) (bool, error) {
	v, ok := opts[key]
	if !ok {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: must be true or false", key, v)
	}
	return b, nil
}

func parseIntOpt(opts map[string]string, key string, def int) (int, error) {
	v, ok := opts[key]
	if !ok {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be a positive integer", key, v)
	}
	return n, nil
}

func parseRegexOpt(opts map[string]string, key string, def string) (*regexp.Regexp, error) {
	v, ok := opts[key]
	if !ok {
		v = def
	}
	if v == "" {
		return nil, nil
	}
	r, err := regexp.Compile(v)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return r, nil
}

func parseArrOpt(opts map[string]string, key string, def []string) []string {
	v, ok := opts[key]
	if !ok || v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
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
