package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"
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
	DaemonName          string            `json:"DaemonName"`
	ContainerEntrypoint string            `json:"ContainerEntrypoint"`
	ContainerArgs       []string          `json:"ContainerArgs"`
}

// tagData provides the template variables available in the tag option.
// Compatible with the built-in Docker log driver tag template variables.
type tagData struct {
	ID           string // Short (12-char) container ID
	FullID       string // Full container ID
	Name         string // Container name (without leading /)
	ImageName    string // Image name
	ImageID      string // Short (12-char) image ID
	ImageFullID  string // Full image ID
	Command      string // Container entrypoint + args
	DaemonName   string // Docker daemon name
}

func newTagData(info *containerInfo) tagData {
	td := tagData{
		FullID:      info.ContainerID,
		Name:        strings.TrimPrefix(info.ContainerName, "/"),
		ImageName:   info.ContainerImageName,
		ImageFullID: info.ContainerImageID,
		DaemonName:  info.DaemonName,
	}
	if len(info.ContainerID) >= 12 {
		td.ID = info.ContainerID[:12]
	}
	if len(info.ContainerImageID) >= 12 {
		// Strip sha256: prefix if present
		imgID := info.ContainerImageID
		imgID = strings.TrimPrefix(imgID, "sha256:")
		if len(imgID) >= 12 {
			td.ImageID = imgID[:12]
		}
	}
	cmd := info.ContainerEntrypoint
	if len(info.ContainerArgs) > 0 {
		cmd += " " + strings.Join(info.ContainerArgs, " ")
	}
	td.Command = cmd
	return td
}

const defaultTagTemplate = "{{.Name}}"

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
	baseVars, err := w.buildBaseVars()
	if err != nil {
		return nil, err
	}
	w.baseVars = baseVars
	return w, nil
}

func (w *journalWriter) buildBaseVars() (map[string]string, error) {
	vars := map[string]string{}

	td := newTagData(&w.info)

	// Container metadata
	vars["CONTAINER_ID"] = td.ID
	vars["CONTAINER_ID_FULL"] = td.FullID
	vars["CONTAINER_NAME"] = td.Name
	vars["IMAGE_NAME"] = td.ImageName

	// Tag: render Go template, default to {{.Name}}
	tag, err := renderTag(w.cfg.Tag, td)
	if err != nil {
		return nil, fmt.Errorf("rendering tag template: %w", err)
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

	return vars, nil
}

// renderTag executes the tag as a Go template against container metadata.
// If the tag is empty, uses defaultTagTemplate. If the tag contains no
// template delimiters, it's used as a literal string.
func renderTag(tagTmpl string, td tagData) (string, error) {
	if tagTmpl == "" {
		tagTmpl = defaultTagTemplate
	}

	// Fast path: no template syntax, use as literal
	if !strings.Contains(tagTmpl, "{{") {
		return tagTmpl, nil
	}

	tmpl, err := template.New("tag").Parse(tagTmpl)
	if err != nil {
		return "", fmt.Errorf("invalid tag template %q: %w", tagTmpl, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, td); err != nil {
		return "", fmt.Errorf("executing tag template %q: %w", tagTmpl, err)
	}
	return buf.String(), nil
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

// sanitizeFieldName converts a string to a valid journal field name.
// Journal field names must be uppercase ASCII letters, digits, or underscores.
func sanitizeFieldName(name string) string {
	var buf strings.Builder
	buf.Grow(len(name))

	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			buf.WriteRune(r - 32) // Convert to uppercase
		} else if r >= 'A' && r <= 'Z' {
			buf.WriteRune(r)
		} else if r >= '0' && r <= '9' {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('_')
		}
	}

	result := buf.String()
	// Ensure first char is not a digit
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		return "_" + result
	}
	return result
}

// Write sends a log entry to journald with optional JSON-extracted fields.
func (w *journalWriter) Write(msg mergedMessage, pri Priority, processedLine []byte, jsonFields map[string]string) error {
	vars := make(map[string]string, len(w.baseVars)+2+len(jsonFields))

	// Add base fields
	for k, v := range w.baseVars {
		vars[k] = v
	}

	// Add JSON fields with JSON_ prefix
	if len(jsonFields) > 0 {
		for k, v := range jsonFields {
			fieldName := "JSON_" + sanitizeFieldName(k)
			vars[fieldName] = v
		}
	}

	// Add timestamp
	ts := time.Unix(0, msg.TimeNano)
	if !ts.IsZero() {
		vars["SYSLOG_TIMESTAMP"] = ts.Format(time.RFC3339Nano)
	}

	// Send to journal
	return w.sendFn(string(processedLine), pri, vars)
}
