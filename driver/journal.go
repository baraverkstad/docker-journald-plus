package driver

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// containerInfo holds parsed container metadata from Docker's Info JSON.
type containerInfo struct {
	Config             map[string]string `json:"Config"`
	ContainerID        string            `json:"ContainerID"`
	ContainerName      string            `json:"ContainerName"`
	ContainerImageID   string            `json:"ContainerImageID"`
	ContainerImageName string            `json:"ContainerImageName"`
	ContainerCreated   time.Time         `json:"ContainerCreated"`
	ContainerEnv       []string          `json:"ContainerEnv"`
	ContainerLabels    map[string]string `json:"ContainerLabels"`
	DaemonName         string            `json:"DaemonName"`
}

// journalWriter handles writing processed messages to journald.
type journalWriter struct {
	cfg      *Config
	info     containerInfo
	baseVars map[string]string // pre-computed journal fields
	sendFn   func(message string, priority Priority, vars map[string]string) error
}

// JournalSendFunc is the function signature for writing to journald.
// In production this is journal.Send; in tests it can be replaced.
type JournalSendFunc func(message string, priority Priority, vars map[string]string) error

func newJournalWriter(cfg *Config, infoJSON json.RawMessage, sendFn JournalSendFunc) (*journalWriter, error) {
	var info containerInfo
	if err := json.Unmarshal(infoJSON, &info); err != nil {
		return nil, fmt.Errorf("parsing container info: %w", err)
	}

	w := &journalWriter{
		cfg:    cfg,
		info:   info,
		sendFn: sendFn,
	}
	w.baseVars = w.buildBaseVars()
	return w, nil
}

func (w *journalWriter) buildBaseVars() map[string]string {
	vars := map[string]string{}

	// Container metadata
	if len(w.info.ContainerID) >= 12 {
		vars["CONTAINER_ID"] = w.info.ContainerID[:12]
	}
	vars["CONTAINER_ID_FULL"] = w.info.ContainerID
	vars["CONTAINER_NAME"] = strings.TrimPrefix(w.info.ContainerName, "/")
	vars["IMAGE_NAME"] = w.info.ContainerImageName

	// Tag: default to container name
	tag := w.cfg.Tag
	if tag == "" {
		tag = strings.TrimPrefix(w.info.ContainerName, "/")
	}
	vars["CONTAINER_TAG"] = tag
	vars["SYSLOG_IDENTIFIER"] = tag

	// Include selected labels
	w.addFilteredFields(vars, w.info.ContainerLabels, w.cfg.Labels, w.cfg.LabelsRegex)

	// Include selected env vars
	envMap := make(map[string]string)
	for _, e := range w.info.ContainerEnv {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
		}
	}
	w.addFilteredFields(vars, envMap, w.cfg.Env, w.cfg.EnvRegex)

	return vars
}

func (w *journalWriter) addFilteredFields(vars map[string]string, source map[string]string, keys []string, re *regexp.Regexp) {
	if len(keys) == 0 && re == nil {
		return
	}

	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	for k, v := range source {
		if keySet[k] || (re != nil && re.MatchString(k)) {
			fieldName := sanitizeFieldName(k)
			vars[fieldName] = v
		}
	}
}

// sanitizeFieldName converts a label/env key to a valid journal field name.
// Journal fields must be uppercase ASCII letters, digits, and underscores.
func sanitizeFieldName(s string) string {
	var b strings.Builder
	for _, c := range strings.ToUpper(s) {
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Write sends a processed message to journald.
func (w *journalWriter) Write(msg mergedMessage, priority Priority, line []byte) error {
	vars := make(map[string]string, len(w.baseVars)+1)
	for k, v := range w.baseVars {
		vars[k] = v
	}

	ts := time.Unix(0, msg.TimeNano)
	if !ts.IsZero() {
		vars["SYSLOG_TIMESTAMP"] = ts.Format(time.RFC3339Nano)
	}

	return w.sendFn(string(line), priority, vars)
}
