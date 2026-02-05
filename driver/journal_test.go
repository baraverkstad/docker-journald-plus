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
