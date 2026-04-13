package driver

import (
	"strings"
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
			wantFields:  map[string]string{"JSON_REQUEST_ID": "123"},
		},
		{
			name:        "valid JSON with severity and msg (alternate keys)",
			line:        `{"severity":"warn","msg":"warning message","user_id":"456"}`,
			wantOK:      true,
			wantLevel:   "warn",
			wantMessage: "warning message",
			wantFields:  map[string]string{"JSON_USER_ID": "456"},
		},
		{
			name:        "Docker json-file format",
			line:        `{"log":"container output\n","stream":"stdout","time":"2024-01-01T00:00:00Z"}`,
			wantOK:      true,
			wantLevel:   "",
			wantMessage: "container output\n",
			wantFields:  map[string]string{"JSON_STREAM": "stdout", "JSON_TIME": "2024-01-01T00:00:00Z"},
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

func TestParseJSONLogSkipKeys(t *testing.T) {
	cfg := &Config{
		ParseJSON:       true,
		JSONLevelKeys:   []string{"level"},
		JSONMessageKeys: []string{"message"},
		JSONSkipKeys:    []string{"ts", "time"},
	}

	parsed, ok := ParseJSONLog(cfg, []byte(`{"level":"info","message":"hello","ts":1234567890,"time":"2024-01-01","request_id":"abc"}`))
	if !ok {
		t.Fatal("ParseJSONLog() returned false, want true")
	}
	if _, found := parsed.ExtraFields["JSON_TS"]; found {
		t.Error("ts should be skipped")
	}
	if _, found := parsed.ExtraFields["JSON_TIME"]; found {
		t.Error("time should be skipped")
	}
	if parsed.ExtraFields["JSON_REQUEST_ID"] != "abc" {
		t.Errorf("JSON_REQUEST_ID = %q, want %q", parsed.ExtraFields["JSON_REQUEST_ID"], "abc")
	}
}

func TestParseJSONLogExtraInline(t *testing.T) {
	cfg := &Config{
		ParseJSON:       true,
		JSONLevelKeys:   []string{"level"},
		JSONMessageKeys: []string{"message"},
		JSONExtraInline: true,
	}

	parsed, ok := ParseJSONLog(cfg, []byte(`{"level":"info","message":"hello","request_id":"abc","count":3}`))
	if !ok {
		t.Fatal("ParseJSONLog() returned false, want true")
	}
	if len(parsed.ExtraFields) != 0 {
		t.Errorf("ExtraFields should be empty in inline mode, got %v", parsed.ExtraFields)
	}
	if parsed.InlineJSON == "" {
		t.Error("InlineJSON should be non-empty in inline mode with remaining fields")
	}
}

func TestParseJSONLogExtraInlineEmpty(t *testing.T) {
	cfg := &Config{
		ParseJSON:       true,
		JSONLevelKeys:   []string{"level"},
		JSONMessageKeys: []string{"message"},
		JSONExtraInline: true,
	}

	// All keys consumed: level + message, nothing left
	parsed, ok := ParseJSONLog(cfg, []byte(`{"level":"info","message":"hello"}`))
	if !ok {
		t.Fatal("ParseJSONLog() returned false, want true")
	}
	if parsed.InlineJSON != "" {
		t.Errorf("InlineJSON should be empty when no keys remain, got %q", parsed.InlineJSON)
	}
}

func TestParseJSONLogSkipKeysWithInline(t *testing.T) {
	cfg := &Config{
		ParseJSON:       true,
		JSONLevelKeys:   []string{"level"},
		JSONMessageKeys: []string{"message"},
		JSONSkipKeys:    []string{"ts"},
		JSONExtraInline: true,
	}

	// ts is skipped, only request_id remains for inline
	parsed, ok := ParseJSONLog(cfg, []byte(`{"level":"info","message":"hello","ts":123,"request_id":"abc"}`))
	if !ok {
		t.Fatal("ParseJSONLog() returned false, want true")
	}
	if parsed.InlineJSON == "" {
		t.Error("InlineJSON should contain request_id")
	}
	// ts must not appear in InlineJSON
	if strings.Contains(parsed.InlineJSON, `"ts"`) {
		t.Errorf("InlineJSON should not contain ts, got %s", parsed.InlineJSON)
	}
}

func TestFlattenJSON(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]any
		want map[string]string
	}{
		{
			name: "string value",
			obj:  map[string]any{"request_id": "abc"},
			want: map[string]string{"JSON_REQUEST_ID": "abc"},
		},
		{
			name: "integer float",
			obj:  map[string]any{"count": float64(42)},
			want: map[string]string{"JSON_COUNT": "42"},
		},
		{
			name: "fractional float",
			obj:  map[string]any{"pi": 3.14},
			want: map[string]string{"JSON_PI": "3.14"},
		},
		{
			name: "boolean",
			obj:  map[string]any{"success": true},
			want: map[string]string{"JSON_SUCCESS": "true"},
		},
		{
			name: "null skipped",
			obj:  map[string]any{"gone": nil},
			want: map[string]string{},
		},
		{
			name: "nested object",
			obj:  map[string]any{"meta": map[string]any{"user": "alice"}},
			want: map[string]string{"JSON_META": `{"user":"alice"}`},
		},
		{
			name: "special chars in key",
			obj:  map[string]any{"http.method": "GET", "user-name": "alice", "123field": "val"},
			want: map[string]string{"JSON_HTTP_METHOD": "GET", "JSON_USER_NAME": "alice", "JSON__123FIELD": "val"},
		},
		{
			name: "empty map",
			obj:  map[string]any{},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenJSON(tt.obj)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d fields, want %d: %v", len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("%s = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
