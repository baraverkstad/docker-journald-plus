package driver

import (
	"testing"
	"time"
)

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.MultilineRegex == nil {
		t.Fatal("MultilineRegex should default to non-nil")
	}
	if !cfg.MultilineRegex.MatchString(" continuation") {
		t.Error("default MultilineRegex should match leading whitespace")
	}
	if cfg.MultilineRegex.MatchString("new line") {
		t.Error("default MultilineRegex should not match non-whitespace start")
	}

	if cfg.MultilineTimeout != 10*time.Millisecond {
		t.Errorf("MultilineTimeout = %v, want 10ms", cfg.MultilineTimeout)
	}
	if cfg.MultilineMaxLines != 100 {
		t.Errorf("MultilineMaxLines = %d, want 100", cfg.MultilineMaxLines)
	}
	if cfg.MultilineMaxBytes != 1048576 {
		t.Errorf("MultilineMaxBytes = %d, want 1048576", cfg.MultilineMaxBytes)
	}
	if cfg.MultilineSep != "\n" {
		t.Errorf("MultilineSep = %q, want %q", cfg.MultilineSep, "\n")
	}

	if !cfg.PriorityPrefix {
		t.Error("PriorityPrefix should default to true")
	}
	if cfg.PriorityDefaultStdout != PriInfo {
		t.Errorf("PriorityDefaultStdout = %d, want %d", cfg.PriorityDefaultStdout, PriInfo)
	}
	if cfg.PriorityDefaultStderr != PriErr {
		t.Errorf("PriorityDefaultStderr = %d, want %d", cfg.PriorityDefaultStderr, PriErr)
	}

	// JSON parsing defaults
	if cfg.ParseJSON {
		t.Error("ParseJSON should default to false")
	}
	if len(cfg.JSONLevelKeys) != 3 || cfg.JSONLevelKeys[0] != "level" || cfg.JSONLevelKeys[1] != "severity" || cfg.JSONLevelKeys[2] != "log_level" {
		t.Errorf("JSONLevelKeys = %v, want [level severity log_level]", cfg.JSONLevelKeys)
	}
	if len(cfg.JSONMessageKeys) != 3 || cfg.JSONMessageKeys[0] != "message" || cfg.JSONMessageKeys[1] != "msg" || cfg.JSONMessageKeys[2] != "log" {
		t.Errorf("JSONMessageKeys = %v, want [message msg log]", cfg.JSONMessageKeys)
	}

	// Default priority matchers should be populated
	if len(cfg.PriorityMatchers) == 0 {
		t.Fatal("PriorityMatchers should have defaults")
	}

	// Check specific defaults exist
	found := map[Priority]bool{}
	for _, m := range cfg.PriorityMatchers {
		found[m.Priority] = true
	}
	for _, pri := range []Priority{PriCrit, PriErr, PriWarning, PriNotice, PriDebug} {
		if !found[pri] {
			t.Errorf("missing default matcher for priority %d", pri)
		}
	}
}

func TestParseConfigDefaultMatchers(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	tests := []struct {
		line    string
		wantPri Priority
	}{
		{"ERROR something broke", PriErr},
		{"FATAL crash", PriErr},
		{"[ERROR] bad request", PriErr},
		{"[Fatal] out of memory", PriErr},
		{"WARN disk space low", PriWarning},
		{"WARNING timeout", PriWarning},
		{"[Warning] slow query", PriWarning},
		{"CRITICAL failure", PriCrit},
		{"[Critical] overload", PriCrit},
		{"[Note] schema updated", PriNotice},
		{"DEBUG tracing", PriDebug},
		{"[Debug] variable dump", PriDebug},
	}

	for _, tt := range tests {
		matched := false
		for _, m := range cfg.PriorityMatchers {
			if m.Regex.MatchString(tt.line) {
				if m.Priority != tt.wantPri {
					t.Errorf("line %q: matched priority %d, want %d", tt.line, m.Priority, tt.wantPri)
				}
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("line %q: no matcher matched, want priority %d", tt.line, tt.wantPri)
		}
	}
}

func TestParseConfigOverrides(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"multiline-regex":         `^\t`,
		"multiline-timeout":       "50ms",
		"multiline-max-lines":     "50",
		"multiline-max-bytes":     "65536",
		"multiline-separator":     " ",
		"priority-prefix":         "false",
		"priority-default-stdout": "debug",
		"priority-default-stderr": "warning",
		"priority-match-err":      "",      // disable default
		"priority-match-info":     "^INFO", // add new
	})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if !cfg.MultilineRegex.MatchString("\tcontinuation") {
		t.Error("MultilineRegex should match tab")
	}
	if cfg.MultilineTimeout != 50*time.Millisecond {
		t.Errorf("MultilineTimeout = %v, want 50ms", cfg.MultilineTimeout)
	}
	if cfg.MultilineMaxLines != 50 {
		t.Errorf("MultilineMaxLines = %d, want 50", cfg.MultilineMaxLines)
	}
	if cfg.MultilineMaxBytes != 65536 {
		t.Errorf("MultilineMaxBytes = %d, want 65536", cfg.MultilineMaxBytes)
	}
	if cfg.MultilineSep != " " {
		t.Errorf("MultilineSep = %q, want %q", cfg.MultilineSep, " ")
	}
	if cfg.PriorityPrefix {
		t.Error("PriorityPrefix should be false")
	}
	if cfg.PriorityDefaultStdout != PriDebug {
		t.Errorf("PriorityDefaultStdout = %d, want %d", cfg.PriorityDefaultStdout, PriDebug)
	}
	if cfg.PriorityDefaultStderr != PriWarning {
		t.Errorf("PriorityDefaultStderr = %d, want %d", cfg.PriorityDefaultStderr, PriWarning)
	}

	// err matcher should be disabled, info should be present
	for _, m := range cfg.PriorityMatchers {
		if m.Priority == PriErr {
			t.Error("PriErr matcher should be disabled by empty string override")
		}
	}
	foundInfo := false
	for _, m := range cfg.PriorityMatchers {
		if m.Priority == PriInfo {
			foundInfo = true
		}
	}
	if !foundInfo {
		t.Error("PriInfo matcher should be present from override")
	}
}

func TestParseConfigDisableMultiline(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"multiline-regex": "",
	})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.MultilineRegex != nil {
		t.Error("MultilineRegex should be nil when set to empty string")
	}
}

func TestParseConfigRejectsUnknown(t *testing.T) {
	_, err := ParseConfig(map[string]string{"bogus": "value"})
	if err == nil {
		t.Fatal("expected error for unknown option")
	}
}

func TestParseConfigJSONOptions(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"parse-json":        "true",
		"json-level-keys":   "lvl,severity",
		"json-message-keys": "text,body",
	})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if !cfg.ParseJSON {
		t.Error("ParseJSON should be true")
	}

	if len(cfg.JSONLevelKeys) != 2 || cfg.JSONLevelKeys[0] != "lvl" || cfg.JSONLevelKeys[1] != "severity" {
		t.Errorf("JSONLevelKeys = %v, want [lvl severity]", cfg.JSONLevelKeys)
	}

	if len(cfg.JSONMessageKeys) != 2 || cfg.JSONMessageKeys[0] != "text" || cfg.JSONMessageKeys[1] != "body" {
		t.Errorf("JSONMessageKeys = %v, want [text body]", cfg.JSONMessageKeys)
	}
}

func TestParseConfigJSONOptionsWithSpaces(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"parse-json":        "true",
		"json-level-keys":   " level , severity , log_level ",
		"json-message-keys": " message , msg , log ",
	})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Should trim spaces
	if len(cfg.JSONLevelKeys) != 3 || cfg.JSONLevelKeys[0] != "level" || cfg.JSONLevelKeys[1] != "severity" || cfg.JSONLevelKeys[2] != "log_level" {
		t.Errorf("JSONLevelKeys = %v, want [level severity log_level]", cfg.JSONLevelKeys)
	}

	if len(cfg.JSONMessageKeys) != 3 || cfg.JSONMessageKeys[0] != "message" || cfg.JSONMessageKeys[1] != "msg" || cfg.JSONMessageKeys[2] != "log" {
		t.Errorf("JSONMessageKeys = %v, want [message msg log]", cfg.JSONMessageKeys)
	}
}

func TestParseConfigRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		opts map[string]string
	}{
		{"bad multiline regex", map[string]string{"multiline-regex": "[invalid"}},
		{"bad timeout", map[string]string{"multiline-timeout": "notaduration"}},
		{"negative timeout", map[string]string{"multiline-timeout": "-5ms"}},
		{"bad max-lines", map[string]string{"multiline-max-lines": "abc"}},
		{"zero max-lines", map[string]string{"multiline-max-lines": "0"}},
		{"bad priority name", map[string]string{"priority-default-stdout": "critical"}},
		{"bad priority prefix", map[string]string{"priority-prefix": "maybe"}},
		{"bad match regex", map[string]string{"priority-match-err": "[broken"}},
		{"bad labels-regex", map[string]string{"labels-regex": "[broken"}},
		{"bad env-regex", map[string]string{"env-regex": "[broken"}},
		{"bad parse-json", map[string]string{"parse-json": "maybe"}},
		{"field extractor no capture group", map[string]string{"field-REQUEST_ID": "request_id=[a-z0-9]+"}},
		{"field extractor bad regex", map[string]string{"field-USER_ID": "[invalid"}},
		{"field extractor empty name", map[string]string{"field-": "pattern"}},
		{"field extractor empty pattern", map[string]string{"field-TEST": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig(tt.opts)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestParseConfigFieldExtractors(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"field-REQUEST_ID": `request_id=([a-z0-9]+)`,
		"field-USER_ID":    `user=(\d+)`,
		"field-TRACE_ID":   `trace[:\s]+([a-f0-9]{32})`,
	})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if len(cfg.FieldExtractors) != 3 {
		t.Fatalf("got %d field extractors, want 3", len(cfg.FieldExtractors))
	}

	// Check field names are present (order may vary due to map iteration)
	names := make(map[string]bool)
	for _, fe := range cfg.FieldExtractors {
		names[fe.FieldName] = true
		if fe.Regex == nil {
			t.Errorf("field %s has nil regex", fe.FieldName)
		}
	}

	for _, want := range []string{"REQUEST_ID", "USER_ID", "TRACE_ID"} {
		if !names[want] {
			t.Errorf("missing field extractor %s", want)
		}
	}
}

func TestExtractFields(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{
		"field-REQUEST_ID": `request_id=([a-z0-9]+)`,
		"field-USER_ID":    `user=(\d+)`,
		"field-SESSION":    `session:([a-f0-9-]{36})`,
	})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	tests := []struct {
		name    string
		message string
		want    map[string]string
	}{
		{
			name:    "single field match",
			message: "Processing request_id=abc123 for endpoint",
			want:    map[string]string{"REQUEST_ID": "abc123"},
		},
		{
			name:    "multiple fields match",
			message: "request_id=xyz789 user=42 completed",
			want:    map[string]string{"REQUEST_ID": "xyz789", "USER_ID": "42"},
		},
		{
			name:    "all fields match",
			message: "request_id=test123 user=999 session:550e8400-e29b-41d4-a716-446655440000 done",
			want: map[string]string{
				"REQUEST_ID": "test123",
				"USER_ID":    "999",
				"SESSION":    "550e8400-e29b-41d4-a716-446655440000",
			},
		},
		{
			name:    "no match",
			message: "simple log line",
			want:    nil,
		},
		{
			name:    "partial match",
			message: "request_id=partial no user",
			want:    map[string]string{"REQUEST_ID": "partial"},
		},
		{
			name:    "first capture group only",
			message: "request_id=abc123def extra text",
			want:    map[string]string{"REQUEST_ID": "abc123def"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ExtractFields(tt.message)
			if len(got) != len(tt.want) {
				t.Errorf("got %d fields, want %d: %v", len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("field %s = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestExtractFieldsNoExtractors(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	result := cfg.ExtractFields("request_id=abc123")
	if result != nil {
		t.Errorf("expected nil when no extractors configured, got %v", result)
	}
}
