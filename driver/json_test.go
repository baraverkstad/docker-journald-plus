package driver

import (
	"testing"
)

func TestParseJSONLog(t *testing.T) {
	cfg := &Config{
		ParseJSON:       true,
		JSONLevelKeys:   []string{"level", "severity"},
		JSONMessageKeys: []string{"message", "msg", "log"},
	}

	tests := []struct {
		name        string
		line        string
		wantOK      bool
		wantLevel   string
		wantMessage string
		wantFields  map[string]string
	}{
		{
			name:        "valid JSON with level and message",
			line:        `{"level":"error","message":"something failed","request_id":"123"}`,
			wantOK:      true,
			wantLevel:   "error",
			wantMessage: "something failed",
			wantFields:  map[string]string{"request_id": "123"},
		},
		{
			name:        "valid JSON with severity and msg (alternate keys)",
			line:        `{"severity":"warn","msg":"warning message","user_id":"456"}`,
			wantOK:      true,
			wantLevel:   "warn",
			wantMessage: "warning message",
			wantFields:  map[string]string{"user_id": "456"},
		},
		{
			name:        "Docker json-file format",
			line:        `{"log":"container output\n","stream":"stdout","time":"2024-01-01T00:00:00Z"}`,
			wantOK:      true,
			wantLevel:   "",
			wantMessage: "container output\n",
			wantFields:  map[string]string{"stream": "stdout", "time": "2024-01-01T00:00:00Z"},
		},
		{
			name:   "not JSON - plain text",
			line:   `plain text log message`,
			wantOK: false,
		},
		{
			name:   "invalid JSON - missing closing brace",
			line:   `{"level":"error","message":"fail"`,
			wantOK: false,
		},
		{
			name:   "JSON array instead of object",
			line:   `["item1", "item2"]`,
			wantOK: false,
		},
		{
			name:        "JSON with numeric and boolean fields",
			line:        `{"message":"test","count":42,"success":true}`,
			wantOK:      true,
			wantMessage: "test",
			wantFields:  map[string]string{"count": "42", "success": "true"},
		},
		{
			name:        "JSON with nested object",
			line:        `{"message":"test","metadata":{"user":"alice","role":"admin"}}`,
			wantOK:      true,
			wantMessage: "test",
			// Don't check exact JSON format since key order is non-deterministic
		},
		{
			name:        "JSON with float values",
			line:        `{"message":"test","pi":3.14,"count":100.0}`,
			wantOK:      true,
			wantMessage: "test",
			wantFields:  map[string]string{"pi": "3.14", "count": "100"},
		},
		{
			name:   "JSON with no message field",
			line:   `{"level":"error","request_id":"123"}`,
			wantOK: false,
		},
		{
			name:   "parse-json disabled",
			line:   `{"level":"error","message":"test"}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCfg := cfg
			if tt.name == "parse-json disabled" {
				testCfg = &Config{
					ParseJSON:       false,
					JSONLevelKeys:   []string{"level", "severity"},
					JSONMessageKeys: []string{"message", "msg", "log"},
				}
			}

			parsed, ok := ParseJSONLog(testCfg, []byte(tt.line))

			if ok != tt.wantOK {
				t.Errorf("ParseJSONLog() ok = %v, want %v", ok, tt.wantOK)
				return
			}

			if !ok {
				return
			}

			if parsed.Level != tt.wantLevel {
				t.Errorf("Level = %q, want %q", parsed.Level, tt.wantLevel)
			}

			if parsed.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", parsed.Message, tt.wantMessage)
			}

			// Only check wantFields if specified (some tests skip it due to non-deterministic JSON ordering)
			if tt.wantFields != nil {
				if len(parsed.ExtraFields) != len(tt.wantFields) {
					t.Errorf("got %d extra fields, want %d", len(parsed.ExtraFields), len(tt.wantFields))
				}

				for k, v := range tt.wantFields {
					if got := parsed.ExtraFields[k]; got != v {
						t.Errorf("ExtraFields[%q] = %q, want %q", k, got, v)
					}
				}
			}
		})
	}
}

func TestJSONLevelToPriority(t *testing.T) {
	tests := []struct {
		level   string
		wantPri Priority
		wantOK  bool
	}{
		{"debug", 7, true},
		{"DEBUG", 7, true},
		{"trace", 7, true},
		{"info", 6, true},
		{"INFO", 6, true},
		{"information", 6, true},
		{"notice", 5, true},
		{"NOTICE", 5, true},
		{"warn", 4, true},
		{"warning", 4, true},
		{"WARNING", 4, true},
		{"error", 3, true},
		{"ERROR", 3, true},
		{"err", 3, true},
		{"fatal", 2, true},
		{"FATAL", 2, true},
		{"critical", 2, true},
		{"CRITICAL", 2, true},
		{"crit", 2, true},
		{"panic", 1, true},
		{"PANIC", 1, true},
		{"alert", 1, true},
		{"emerg", 0, true},
		{"emergency", 0, true},
		{"EMERGENCY", 0, true},
		{"unknown", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			pri, ok := JSONLevelToPriority(tt.level)
			if ok != tt.wantOK {
				t.Errorf("JSONLevelToPriority(%q) ok = %v, want %v", tt.level, ok, tt.wantOK)
			}
			if ok && pri != tt.wantPri {
				t.Errorf("JSONLevelToPriority(%q) = %d, want %d", tt.level, pri, tt.wantPri)
			}
		})
	}
}

func TestSanitizeFieldName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"request_id", "REQUEST_ID"},
		{"user-name", "USER_NAME"},
		{"http.method", "HTTP_METHOD"},
		{"123field", "_123FIELD"},
		{"MixedCase", "MIXEDCASE"},
		{"with spaces", "WITH_SPACES"},
		{"special!@#chars", "SPECIAL___CHARS"},
		{"_leading_underscore", "_LEADING_UNDERSCORE"},
		{"ALREADY_UPPER", "ALREADY_UPPER"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFieldName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFieldName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
