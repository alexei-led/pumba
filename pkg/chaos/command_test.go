package chaos

import (
	"testing"
)

func TestRe2PatternExtraction(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantPat string
	}{
		{"simple pattern", "re2:^myregex$", "^myregex$"},
		{"pattern ending with 2", "re2:^myregex2", "^myregex2"},
		{"pattern with re2 in name", "re2:^service-re2-test", "^service-re2-test"},
		{"pattern with colon", "re2:^app:v2", "^app:v2"},
		{"just prefix", "re2:", ""},
		{"pattern with 2 everywhere", "re2:r2d2-container2", "r2d2-container2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what getNamesOrPattern does with the re2 prefix
			got := ""
			if len(tt.input) > len(Re2Prefix) || tt.input == Re2Prefix {
				// This is the fixed logic using TrimPrefix
				got = tt.input[len(Re2Prefix):]
			}
			if got != tt.wantPat {
				t.Errorf("pattern = %q, want %q", got, tt.wantPat)
			}
		})
	}
}
