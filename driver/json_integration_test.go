package driver

import (
	"encoding/json"
	"testing"
)

// TestJSONParsingIntegration tests that JSON fields are properly extracted and passed to journald.
func TestJSONParsingIntegration(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"parse-json": "true",
	})

	infoJSON, _ := json.Marshal(containerInfo{
		ContainerID:   "test123",
		ContainerName: "/testcontainer",
	})

	var lastMsg string
	var lastPri Priority
	var lastVars map[string]string

	sendFn := func(message string, priority Priority, vars map[string]string) error {
		lastMsg = message
		lastPri = priority
		lastVars = vars
		return nil
	}

	w, err := newJournalWriter(cfg, json.RawMessage(infoJSON), sendFn)
	if err != nil {
		t.Fatalf("newJournalWriter: %v", err)
	}

	// Test 1: JSON log with level and extra fields
	jsonLog := `{"level":"error","message":"database connection failed","request_id":"abc123","retry_count":3}`
	msg := mergedMessage{Line: []byte(jsonLog), Source: "stdout", TimeNano: 1000}

	// Simulate the pipeline processing
	parsed, ok := ParseJSONLog(cfg, msg.Line)
	if !ok {
		t.Fatal("Expected JSON to be parsed")
	}

	priority, _ := JSONLevelToPriority(parsed.Level)
	err = w.Write(msg, priority, []byte(parsed.Message), parsed.ExtraFields)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify message
	if lastMsg != "database connection failed" {
		t.Errorf("message = %q, want %q", lastMsg, "database connection failed")
	}

	// Verify priority
	if lastPri != PriErr {
		t.Errorf("priority = %d, want %d (PriErr)", lastPri, PriErr)
	}

	// Verify JSON fields are present with JSON_ prefix
	if lastVars["JSON_REQUEST_ID"] != "abc123" {
		t.Errorf("JSON_REQUEST_ID = %q, want %q", lastVars["JSON_REQUEST_ID"], "abc123")
	}
	if lastVars["JSON_RETRY_COUNT"] != "3" {
		t.Errorf("JSON_RETRY_COUNT = %q, want %q", lastVars["JSON_RETRY_COUNT"], "3")
	}

	// Verify level and message are NOT duplicated in extra fields
	if _, exists := lastVars["JSON_LEVEL"]; exists {
		t.Error("JSON_LEVEL should not be in vars (it was extracted)")
	}
	if _, exists := lastVars["JSON_MESSAGE"]; exists {
		t.Error("JSON_MESSAGE should not be in vars (it was extracted)")
	}

	// Test 2: Non-JSON log falls back to raw text
	plainLog := "plain text error message"
	msg = mergedMessage{Line: []byte(plainLog), Source: "stderr", TimeNano: 2000}

	parsed, ok = ParseJSONLog(cfg, msg.Line)
	if ok {
		t.Fatal("Expected plain text to not be parsed as JSON")
	}

	// Use default priority detection
	pri, line := DetectPriority(cfg, msg.Line, msg.Source)
	err = w.Write(msg, pri, line, nil)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if lastMsg != plainLog {
		t.Errorf("message = %q, want %q", lastMsg, plainLog)
	}

	// stderr should default to PriErr
	if lastPri != PriErr {
		t.Errorf("priority = %d, want %d (PriErr for stderr)", lastPri, PriErr)
	}
}

// TestJSONFieldSanitization verifies that special characters in JSON keys are properly sanitized.
func TestJSONFieldSanitization(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"parse-json": "true",
	})

	infoJSON, _ := json.Marshal(containerInfo{
		ContainerID:   "test123",
		ContainerName: "/testcontainer",
	})

	var lastVars map[string]string
	sendFn := func(message string, priority Priority, vars map[string]string) error {
		lastVars = vars
		return nil
	}

	w, err := newJournalWriter(cfg, json.RawMessage(infoJSON), sendFn)
	if err != nil {
		t.Fatalf("newJournalWriter: %v", err)
	}

	// JSON with special characters in keys
	jsonLog := `{"message":"test","http.method":"GET","user-name":"alice","123field":"value"}`
	msg := mergedMessage{Line: []byte(jsonLog), Source: "stdout", TimeNano: 1000}

	parsed, ok := ParseJSONLog(cfg, msg.Line)
	if !ok {
		t.Fatal("Expected JSON to be parsed")
	}

	err = w.Write(msg, PriInfo, []byte(parsed.Message), parsed.ExtraFields)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify field names are sanitized
	tests := []struct {
		field string
		want  string
	}{
		{"JSON_HTTP_METHOD", "GET"},
		{"JSON_USER_NAME", "alice"},
		{"JSON__123FIELD", "value"}, // Leading digit gets underscore prefix
	}

	for _, tt := range tests {
		if got := lastVars[tt.field]; got != tt.want {
			t.Errorf("%s = %q, want %q", tt.field, got, tt.want)
		}
	}
}
