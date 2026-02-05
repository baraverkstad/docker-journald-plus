package driver

import (
	"regexp"
	"testing"
)

func compileDefaults(t *testing.T) []*regexp.Regexp {
	t.Helper()
	patterns, err := compileTimestampPatterns(defaultTimestampPatterns)
	if err != nil {
		t.Fatalf("compileTimestampPatterns: %v", err)
	}
	return patterns
}

func TestStripTimestampISO8601(t *testing.T) {
	patterns := compileDefaults(t)

	tests := []struct {
		name string
		line string
		want string
	}{
		{"basic ISO", "2024-01-15T10:30:45 ERROR something", "ERROR something"},
		{"millis dot", "2024-01-15T10:30:45.123 ERROR something", "ERROR something"},
		{"millis comma", "2024-01-15 10:30:45,123 ERROR something", "ERROR something"},
		{"micros", "2024-01-15T10:30:45.123456 ERROR something", "ERROR something"},
		{"nanos", "2024-01-15T10:30:45.123456789Z ERROR something", "ERROR something"},
		{"timezone +", "2024-01-15T10:30:45+02:00 ERROR something", "ERROR something"},
		{"timezone compact", "2024-01-15T10:30:45+0200 ERROR something", "ERROR something"},
		{"UTC suffix", "2024-01-15 10:30:45.123 UTC ERROR something", "ERROR something"},
		// CET and other short timezone names are intentionally NOT stripped
		// to avoid matching log level words (ERROR, WARN, INFO, DEBUG).
		// Only Z, UTC, GMT are recognized as timezone suffixes.
		{"CET suffix (partial strip)", "2024-01-15 10:30:45.123 CET ERROR something", "CET ERROR something"},
		{"space separator", "2024-01-15 10:30:45 ERROR something", "ERROR something"},
		{"bracketed", "[2024-01-15 10:30:45] ERROR something", "ERROR something"},
		{"dash separator", "2024-01-15T10:30:45 - ERROR something", "ERROR something"},
		{"pipe separator", "2024-01-15T10:30:45 | ERROR something", "ERROR something"},
		{"colon separator", "2024-01-15T10:30:45: ERROR something", "ERROR something"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripTimestamp([]byte(tt.line), patterns)
			if string(got) != tt.want {
				t.Errorf("StripTimestamp(%q) = %q, want %q", tt.line, string(got), tt.want)
			}
		})
	}
}

func TestStripTimestampGoLog(t *testing.T) {
	patterns := compileDefaults(t)

	tests := []struct {
		line string
		want string
	}{
		{"2024/01/15 10:30:45 message here", "message here"},
		{"2024/01/15 10:30:45.123456 message here", "message here"},
	}

	for _, tt := range tests {
		got := StripTimestamp([]byte(tt.line), patterns)
		if string(got) != tt.want {
			t.Errorf("StripTimestamp(%q) = %q, want %q", tt.line, string(got), tt.want)
		}
	}
}

func TestStripTimestampSyslog(t *testing.T) {
	patterns := compileDefaults(t)

	tests := []struct {
		line string
		want string
	}{
		{"Jan 15 10:30:45 myhost message", "myhost message"},
		{"Jan  5 10:30:45 myhost message", "myhost message"},
		{"Dec 31 23:59:59 message", "message"},
	}

	for _, tt := range tests {
		got := StripTimestamp([]byte(tt.line), patterns)
		if string(got) != tt.want {
			t.Errorf("StripTimestamp(%q) = %q, want %q", tt.line, string(got), tt.want)
		}
	}
}

func TestStripTimestampApacheCLF(t *testing.T) {
	patterns := compileDefaults(t)

	tests := []struct {
		line string
		want string
	}{
		{"15/Oct/2024:10:30:45 +0200 GET /index.html", "GET /index.html"},
		{"[15/Oct/2024:10:30:45 +0200] GET /index.html", "GET /index.html"},
	}

	for _, tt := range tests {
		got := StripTimestamp([]byte(tt.line), patterns)
		if string(got) != tt.want {
			t.Errorf("StripTimestamp(%q) = %q, want %q", tt.line, string(got), tt.want)
		}
	}
}

func TestStripTimestampLog4jDate(t *testing.T) {
	patterns := compileDefaults(t)

	got := StripTimestamp([]byte("14 Nov 2017 20:30:20,434 INFO message"), patterns)
	if string(got) != "INFO message" {
		t.Errorf("got %q, want %q", string(got), "INFO message")
	}
}

func TestStripTimestampApacheError(t *testing.T) {
	patterns := compileDefaults(t)

	got := StripTimestamp([]byte("Wed Oct 15 19:41:46.123456 2019 [error] message"), patterns)
	if string(got) != "[error] message" {
		t.Errorf("got %q, want %q", string(got), "[error] message")
	}
}

func TestStripTimestampNoMatch(t *testing.T) {
	patterns := compileDefaults(t)

	tests := []string{
		"ERROR no timestamp here",
		"just a plain message",
		"[Warning] not a timestamp",
		"12345 not a timestamp",
		"",
	}

	for _, line := range tests {
		got := StripTimestamp([]byte(line), patterns)
		if string(got) != line {
			t.Errorf("StripTimestamp(%q) = %q, should be unchanged", line, string(got))
		}
	}
}

func TestStripTimestampOnlyTimestamp(t *testing.T) {
	patterns := compileDefaults(t)

	// If the entire line is just a timestamp, strip it and return empty
	got := StripTimestamp([]byte("2024-01-15T10:30:45"), patterns)
	if string(got) != "" {
		t.Errorf("got %q, want empty string when line is only timestamp", string(got))
	}
}

func TestStripTimestampCustomPattern(t *testing.T) {
	// MySQL 5.6 short format: 230515 14:30:45
	patterns, err := compileTimestampPatterns([]string{`^\d{6} \d{2}:\d{2}:\d{2}`})
	if err != nil {
		t.Fatal(err)
	}

	got := StripTimestamp([]byte("230515 14:30:45 [Note] message"), patterns)
	if string(got) != "[Note] message" {
		t.Errorf("got %q, want %q", string(got), "[Note] message")
	}
}
