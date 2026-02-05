package driver

import (
	"encoding/json"
	"testing"
)

func TestJournalWriterBaseVars(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"labels": "app,version",
	})

	infoJSON, _ := json.Marshal(containerInfo{
		ContainerID:        "abcdef123456789012345678",
		ContainerName:      "/mycontainer",
		ContainerImageName: "myimage:latest",
		ContainerLabels: map[string]string{
			"app":     "web",
			"version": "1.0",
			"other":   "ignored",
		},
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

	msg := mergedMessage{Line: []byte("hello"), Source: "stdout", TimeNano: 1000000000}
	if err := w.Write(msg, PriInfo, []byte("hello"), nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if lastMsg != "hello" {
		t.Errorf("message = %q", lastMsg)
	}
	if lastPri != PriInfo {
		t.Errorf("priority = %d", lastPri)
	}
	if lastVars["CONTAINER_ID"] != "abcdef123456" {
		t.Errorf("CONTAINER_ID = %q", lastVars["CONTAINER_ID"])
	}
	if lastVars["CONTAINER_NAME"] != "mycontainer" {
		t.Errorf("CONTAINER_NAME = %q", lastVars["CONTAINER_NAME"])
	}
	if lastVars["SYSLOG_IDENTIFIER"] != "mycontainer" {
		t.Errorf("SYSLOG_IDENTIFIER = %q (should default to container name)", lastVars["SYSLOG_IDENTIFIER"])
	}
	if lastVars["IMAGE_NAME"] != "myimage:latest" {
		t.Errorf("IMAGE_NAME = %q", lastVars["IMAGE_NAME"])
	}
	if lastVars["APP"] != "web" {
		t.Errorf("APP label = %q", lastVars["APP"])
	}
	if lastVars["VERSION"] != "1.0" {
		t.Errorf("VERSION label = %q", lastVars["VERSION"])
	}
	if _, ok := lastVars["OTHER"]; ok {
		t.Error("OTHER label should not be included")
	}
}

func TestJournalWriterCustomTag(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"tag": "myapp",
	})

	infoJSON, _ := json.Marshal(containerInfo{
		ContainerID:   "abcdef123456789012345678",
		ContainerName: "/container1",
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

	msg := mergedMessage{Line: []byte("x"), Source: "stdout", TimeNano: 1000}
	w.Write(msg, PriInfo, []byte("x"), nil)

	if lastVars["SYSLOG_IDENTIFIER"] != "myapp" {
		t.Errorf("SYSLOG_IDENTIFIER = %q, want %q", lastVars["SYSLOG_IDENTIFIER"], "myapp")
	}
	if lastVars["CONTAINER_TAG"] != "myapp" {
		t.Errorf("CONTAINER_TAG = %q, want %q", lastVars["CONTAINER_TAG"], "myapp")
	}
}

func TestJournalWriterTagTemplate(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantTag string
	}{
		{"default (empty) uses container name", "", "mycontainer"},
		{"literal string", "myapp", "myapp"},
		{"template {{.ID}}", "{{.ID}}", "abcdef123456"},
		{"template {{.FullID}}", "{{.FullID}}", "abcdef123456789012345678"},
		{"template {{.Name}}", "{{.Name}}", "mycontainer"},
		{"template {{.ImageName}}", "{{.ImageName}}", "myimage:latest"},
		{"composite template", "{{.Name}}/{{.ID}}", "mycontainer/abcdef123456"},
		{"template {{.DaemonName}}", "{{.DaemonName}}/{{.Name}}", "docker/mycontainer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustConfig(t, map[string]string{})
			if tt.tag != "" {
				cfg.Tag = tt.tag
			}

			infoJSON, _ := json.Marshal(containerInfo{
				ContainerID:        "abcdef123456789012345678",
				ContainerName:      "/mycontainer",
				ContainerImageName: "myimage:latest",
				ContainerImageID:   "sha256:deadbeef123456789012345678",
				DaemonName:         "docker",
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

			w.Write(mergedMessage{Line: []byte("x"), Source: "stdout", TimeNano: 1000}, PriInfo, []byte("x"), nil)

			if lastVars["SYSLOG_IDENTIFIER"] != tt.wantTag {
				t.Errorf("SYSLOG_IDENTIFIER = %q, want %q", lastVars["SYSLOG_IDENTIFIER"], tt.wantTag)
			}
			if lastVars["CONTAINER_TAG"] != tt.wantTag {
				t.Errorf("CONTAINER_TAG = %q, want %q", lastVars["CONTAINER_TAG"], tt.wantTag)
			}
		})
	}
}

func TestJournalWriterFieldExtraction(t *testing.T) {
	cfg := mustConfig(t, map[string]string{
		"field-REQUEST_ID": `request_id=([a-z0-9]+)`,
		"field-USER_ID":    `user=(\d+)`,
	})

	infoJSON, _ := json.Marshal(containerInfo{
		ContainerID:   "abcdef123456789012345678",
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

	// Test with both fields present
	msg := mergedMessage{Line: []byte("test"), Source: "stdout", TimeNano: 1000}
	processedLine := []byte("Processing request_id=abc123 for user=42")
	if err := w.Write(msg, PriInfo, processedLine, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if lastVars["REQUEST_ID"] != "abc123" {
		t.Errorf("REQUEST_ID = %q, want %q", lastVars["REQUEST_ID"], "abc123")
	}
	if lastVars["USER_ID"] != "42" {
		t.Errorf("USER_ID = %q, want %q", lastVars["USER_ID"], "42")
	}

	// Test with only one field present
	processedLine = []byte("Log line request_id=xyz789 without user")
	if err := w.Write(msg, PriInfo, processedLine, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if lastVars["REQUEST_ID"] != "xyz789" {
		t.Errorf("REQUEST_ID = %q, want %q", lastVars["REQUEST_ID"], "xyz789")
	}
	if _, ok := lastVars["USER_ID"]; ok {
		t.Errorf("USER_ID should not be present, got %q", lastVars["USER_ID"])
	}

	// Test with no fields matching
	processedLine = []byte("Simple log line")
	if err := w.Write(msg, PriInfo, processedLine, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, ok := lastVars["REQUEST_ID"]; ok {
		t.Errorf("REQUEST_ID should not be present, got %q", lastVars["REQUEST_ID"])
	}
	if _, ok := lastVars["USER_ID"]; ok {
		t.Errorf("USER_ID should not be present, got %q", lastVars["USER_ID"])
	}
}
