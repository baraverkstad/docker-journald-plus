package driver

import "testing"

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"tabs", "INFO\t\tgot renewal info\t\t{}", "INFO got renewal info {}"},
		{"double space", "INFO  double space", "INFO double space"},
		{"single space unchanged", "single space unchanged", "single space unchanged"},
		{"mixed", "mixed\t  tabs and spaces", "mixed tabs and spaces"},
		{"tab in continuation", "first\n\tsecond", "first\n second"},
		{"no whitespace", "no whitespace", "no whitespace"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeWhitespace([]byte(tt.line))
			if string(got) != tt.want {
				t.Errorf("NormalizeWhitespace(%q) = %q, want %q", tt.line, string(got), tt.want)
			}
		})
	}
}

func TestNormalizeWhitespaceDisabledByDefault(t *testing.T) {
	cfg, err := ParseConfig(map[string]string{})
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.NormalizeWhitespace {
		t.Error("NormalizeWhitespace should default to false")
	}
}
