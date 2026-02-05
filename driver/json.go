package driver

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONParsedLog represents a successfully parsed JSON log line.
type JSONParsedLog struct {
	Level       string            // Extracted level/severity value
	Message     string            // Extracted message body
	ExtraFields map[string]string // Other fields to add as JSON_*
}

// ParseJSONLog attempts to parse a log line as JSON.
// Returns (parsed result, true) if successful, (nil, false) if not JSON or parsing fails.
func ParseJSONLog(cfg *Config, line []byte) (*JSONParsedLog, bool) {
	if !cfg.ParseJSON || len(line) == 0 {
		return nil, false
	}

	// Try to unmarshal as JSON object
	var obj map[string]interface{}
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

	// Flatten remaining fields
	for k, v := range obj {
		// Convert value to string
		var strVal string
		switch val := v.(type) {
		case string:
			strVal = val
		case float64:
			strVal = formatFloat(val)
		case bool:
			strVal = formatBool(val)
		case nil:
			continue // Skip null values
		default:
			// For nested objects/arrays, marshal back to JSON string
			if b, err := json.Marshal(val); err == nil {
				strVal = string(b)
			} else {
				continue // Skip if can't marshal
			}
		}

		result.ExtraFields[k] = strVal
	}

	return result, true
}

// JSONLevelToPriority maps a JSON level string to a syslog priority.
// Returns (priority, true) if recognized, (0, false) if not.
func JSONLevelToPriority(level string) (Priority, bool) {
	switch strings.ToLower(level) {
	case "debug", "trace":
		return 7, true // LOG_DEBUG
	case "info", "information":
		return 6, true // LOG_INFO
	case "notice":
		return 5, true // LOG_NOTICE
	case "warn", "warning":
		return 4, true // LOG_WARNING
	case "error", "err":
		return 3, true // LOG_ERR
	case "fatal", "critical", "crit":
		return 2, true // LOG_CRIT
	case "panic", "alert":
		return 1, true // LOG_ALERT
	case "emerg", "emergency":
		return 0, true // LOG_EMERG
	default:
		return 0, false
	}
}

func formatFloat(f float64) string {
	// If integer, format without decimal
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
