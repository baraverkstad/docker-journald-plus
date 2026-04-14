package driver

import (
	"testing"
)

func TestParsePriorityName(t *testing.T) {
	tests := []struct {
		input   string
		wantPri Priority
		wantErr bool
	}{
		{"emerg", PriEmerg, false},
		{"emergency", PriEmerg, false},
		{"EMERGENCY", PriEmerg, false},
		{"alert", PriAlert, false},
		{"panic", PriAlert, false},
		{"PANIC", PriAlert, false},
		{"crit", PriCrit, false},
		{"critical", PriCrit, false},
		{"CRITICAL", PriCrit, false},
		{"fatal", PriCrit, false},
		{"FATAL", PriCrit, false},
		{"err", PriErr, false},
		{"error", PriErr, false},
		{"ERROR", PriErr, false},
		{"warning", PriWarning, false},
		{"warn", PriWarning, false},
		{"WARNING", PriWarning, false},
		{"notice", PriNotice, false},
		{"NOTICE", PriNotice, false},
		{"info", PriInfo, false},
		{"information", PriInfo, false},
		{"INFO", PriInfo, false},
		{"debug", PriDebug, false},
		{"trace", PriDebug, false},
		{"DEBUG", PriDebug, false},
		{"unknown", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pri, err := parsePriorityName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePriorityName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && pri != tt.wantPri {
				t.Errorf("parsePriorityName(%q) = %d, want %d", tt.input, pri, tt.wantPri)
			}
		})
	}
}

func mustConfig(t *testing.T, opts map[string]string) *Config {
	t.Helper()
	cfg, err := ParseConfig(opts)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	return cfg
}

func TestDetectPrioritySdDaemon(t *testing.T) {
	cfg := mustConfig(t, map[string]string{})

	tests := []struct {
		line    string
		wantPri Priority
		wantMsg string
	}{
		{"<3>Error occurred", PriErr, "Error occurred"},
		{"<7>trace detail", PriDebug, "trace detail"},
		{"<0>kernel panic", PriEmerg, "kernel panic"},
		{"<6>informational", PriInfo, "informational"},
	}

	for _, tt := range tests {
		pri, msg := DetectPriority(cfg, []byte(tt.line), "stdout")
		if pri != tt.wantPri {
			t.Errorf("line %q: priority = %d, want %d", tt.line, pri, tt.wantPri)
		}
		if string(msg) != tt.wantMsg {
			t.Errorf("line %q: msg = %q, want %q", tt.line, string(msg), tt.wantMsg)
		}
	}
}

func TestDetectPrioritySdDaemonDisabled(t *testing.T) {
	cfg := mustConfig(t, map[string]string{"priority-prefix": "false"})

	pri, msg := DetectPriority(cfg, []byte("<3>Error occurred"), "stdout")
	// Should not strip prefix, should fall through to pattern matching (ERROR not present) then default
	if string(msg) != "<3>Error occurred" {
		t.Errorf("msg = %q, want unstripped", string(msg))
	}
	// No pattern matches "<3>Error..." so should get default stdout priority
	if pri != PriInfo {
		t.Errorf("priority = %d, want %d (default stdout)", pri, PriInfo)
	}
}

func TestDetectPriorityPatternMatch(t *testing.T) {
	cfg := mustConfig(t, map[string]string{})

	tests := []struct {
		line    string
		source  string
		wantPri Priority
	}{
		{"ERROR something broke", "stdout", PriErr},
		{"FATAL crash", "stdout", PriErr},
		{"[ERROR] bad request", "stdout", PriErr},
		{"[Fatal] out of memory", "stdout", PriErr},
		{"WARNING timeout", "stdout", PriWarning},
		{"WARN disk low", "stdout", PriWarning},
		{"[Warning] slow query", "stdout", PriWarning},
		{"CRITICAL overload", "stdout", PriCrit},
		{"[Note] schema change", "stdout", PriInfo},
		{"DEBUG trace", "stdout", PriDebug},
		{"[Debug] dump", "stdout", PriDebug},
	}

	for _, tt := range tests {
		pri, msg := DetectPriority(cfg, []byte(tt.line), tt.source)
		if pri != tt.wantPri {
			t.Errorf("line %q: priority = %d, want %d", tt.line, pri, tt.wantPri)
		}
		if string(msg) != tt.line {
			t.Errorf("line %q: message was modified to %q", tt.line, string(msg))
		}
	}
}

func TestDetectPriorityDefault(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		// Disable all matchers
		"priority-prefix":        "false",
		"priority-match-crit":    "",
		"priority-match-err":     "",
		"priority-match-warning": "",
		"priority-match-notice":  "",
		"priority-match-debug":   "",
	})

	pri, _ := DetectPriority(cfg, []byte("just a message"), "stdout")
	if pri != PriInfo {
		t.Errorf("stdout default: priority = %d, want %d", pri, PriInfo)
	}

	pri, _ = DetectPriority(cfg, []byte("just a message"), "stderr")
	if pri != PriErr {
		t.Errorf("stderr default: priority = %d, want %d", pri, PriErr)
	}
}

func TestDetectPrioritySdDaemonBeforePattern(t *testing.T) {
	cfg := mustConfig(t, map[string]string{})

	// sd-daemon prefix should take precedence over pattern matching
	// <6> = INFO, but line also starts with ERROR
	pri, msg := DetectPriority(cfg, []byte("<6>ERROR in module"), "stdout")
	if pri != PriInfo {
		t.Errorf("priority = %d, want %d (sd-daemon should win)", pri, PriInfo)
	}
	if string(msg) != "ERROR in module" {
		t.Errorf("msg = %q, want %q", string(msg), "ERROR in module")
	}
}

func TestDetectPriorityWithPrefix(t *testing.T) {
	cfg := mustConfig(t, map[string]string{"priority-prefix": "false"})

	tests := []struct {
		line    string
		source  string
		wantPri Priority
	}{
		// MariaDB style: timestamp stripped but numeric prefix remains
		{" 0 [Note] InnoDB: Buffer pool size", "stdout", PriInfo},
		{" 1 [Warning] InnoDB: Disk full", "stdout", PriWarning},
		{" 0 [ERROR] Connection refused", "stdout", PriErr},

		// Other prefixes
		{"123 ERROR failed to connect", "stdout", PriErr},
		{"  WARN deprecated API", "stdout", PriWarning},
		{"[2026-02-15] CRITICAL system failure", "stdout", PriCrit},
		{"thread-42 DEBUG entering function", "stdout", PriDebug},
		{"pid:1234 FATAL cannot recover", "stdout", PriErr},

		// Longer prefixes (up to 30 chars should work)
		{"very-long-prefix-here-12345 ERROR timeout", "stdout", PriErr},
	}

	for _, tt := range tests {
		pri, msg := DetectPriority(cfg, []byte(tt.line), tt.source)
		if pri != tt.wantPri {
			t.Errorf("line %q: priority = %d, want %d", tt.line, pri, tt.wantPri)
		}
		if string(msg) != tt.line {
			t.Errorf("line %q: message was modified to %q", tt.line, string(msg))
		}
	}
}

func TestDetectPriorityPrefixTooLong(t *testing.T) {
	cfg := mustConfig(t, map[string]string{"priority-prefix": "false"})

	// Prefix longer than 30 chars should not match, falls through to default
	line := "this-is-a-very-long-prefix-that-exceeds-thirty-characters ERROR occurred"
	pri, _ := DetectPriority(cfg, []byte(line), "stdout")
	if pri != PriInfo {
		t.Errorf("priority = %d, want %d (should fall through to default)", pri, PriInfo)
	}
}

func TestStripPriority(t *testing.T) {
	tests := []struct {
		line    string
		opts    map[string]string
		wantNil bool
		want    string
	}{
		{
			line:    "INFO something",
			opts:    map[string]string{},
			wantNil: true,
			want:    "INFO something",
		},
		{
			line: "INFO something",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "WARN something",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "ERROR something",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "[INFO] something",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "[Error] something",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "[Debug] something",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "INFO: something",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "INFO\t\tsomething",
			opts: map[string]string{"strip-priority": "true"},
			want: "something",
		},
		{
			line: "just a message",
			opts: map[string]string{"strip-priority": "true"},
			want: "just a message",
		},
		{
			line: "INFO first\n  continuation",
			opts: map[string]string{"strip-priority": "true"},
			want: "first\n  continuation",
		},
		{
			line: " 0 [Note] InnoDB: thing",
			opts: map[string]string{
				"strip-priority":       "true",
				"strip-priority-regex": `\d+ \[Note\] `,
			},
			want: "InnoDB: thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			cfg := mustConfig(t, tt.opts)
			if tt.wantNil && cfg.StripPriorityRegex != nil {
				t.Errorf("StripPriorityRegex = %v, want nil", cfg.StripPriorityRegex)
			}
			got := string(StripPriority([]byte(tt.line), cfg.StripPriorityRegex))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
