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
