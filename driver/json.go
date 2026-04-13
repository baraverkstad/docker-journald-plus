package driver

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// JSONParsedLog represents a successfully parsed JSON log line.
type JSONParsedLog struct {
	Level       string            // Extracted level/severity value
	Message     string            // Extracted message body
	ExtraFields map[string]string // Other fields to add as JSON_* (when JSONExtraInline=false)
	InlineJSON  string            // Remaining fields marshalled as JSON (when JSONExtraInline=true)
}

// ParseJSONLog attempts to parse a log line as JSON.
// Returns (parsed result, true) if successful, (nil, false) if not JSON or parsing fails.
func ParseJSONLog(cfg *Config, line []byte) (*JSONParsedLog, bool) {
	if !cfg.ParseJSON || len(line) == 0 {
		return nil, false
	}

	// Try to unmarshal as JSON object
	var obj map[string]any
	if err := json.Unmarshal(line, &obj); err != nil {
		return nil, false
	}

	result := &JSONParsedLog{
		ExtraFields: make(map[string]string),
	}

	// Extract level/severity (first match wins)
	for _, key := range cfg.JSONLevelKeys {
		if val, ok := obj[key]; ok {
			if str, ok := val.(string); ok {
				result.Level = str
				delete(obj, key) // Don't duplicate in extra fields
				break
			}
		}
	}

	// Extract message (first match wins)
	for _, key := range cfg.JSONMessageKeys {
		if val, ok := obj[key]; ok {
			if str, ok := val.(string); ok {
				result.Message = str
				delete(obj, key) // Don't duplicate in extra fields
				break
			}
		}
	}

	// If no message found, use empty string (will fall back to original line in caller)
	if result.Message == "" {
		return nil, false
	}

	// Remove skip keys before processing remaining fields
	for _, key := range cfg.JSONSkipKeys {
		delete(obj, key)
	}

	if cfg.JSONExtraInline {
		// Marshal remaining fields as JSON and store for appending to message
		if len(obj) > 0 {
			if b, err := json.Marshal(obj); err == nil {
				result.InlineJSON = string(b)
			}
		}
	} else {
		result.ExtraFields = flattenJSON(obj)
	}

	return result, true
}

// flattenJSON converts a JSON object into journal-friendly key/value pairs
// with sanitized, JSON_-prefixed field names.
func flattenJSON(obj map[string]any) map[string]string {
	fields := make(map[string]string, len(obj))
	for k, v := range obj {
		var strVal string
		switch val := v.(type) {
		case string:
			strVal = val
		case float64:
			if val == float64(int64(val)) {
				strVal = fmt.Sprintf("%d", int64(val))
			} else {
				strVal = fmt.Sprintf("%g", val)
			}
		case bool:
			strVal = strconv.FormatBool(val)
		case nil:
			continue
		default:
			if b, err := json.Marshal(val); err == nil {
				strVal = string(b)
			} else {
				continue
			}
		}
		fields["JSON_"+sanitizeFieldName(k)] = strVal
	}
	return fields
}
