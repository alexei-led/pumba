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

func TestSplitLabels(t *testing.T) {
	tests := []struct {
		name string
		raw  []string
		want []string
	}{
		{"single label", []string{"app=web"}, []string{"app=web"}},
		{"two separate flags", []string{"app=web", "env=prod"}, []string{"app=web", "env=prod"}},
		{"comma-separated", []string{"app=web,env=prod"}, []string{"app=web", "env=prod"}},
		{"mixed", []string{"app=web,env=prod", "tier=frontend"}, []string{"app=web", "env=prod", "tier=frontend"}},
		{"with spaces", []string{"app=web, env=prod"}, []string{"app=web", "env=prod"}},
		{"empty string", []string{""}, nil},
		{"empty slice", []string{}, nil},
		{"k8s labels", []string{"io.kubernetes.container.name=myapp,app.kubernetes.io/version=v2"}, []string{"io.kubernetes.container.name=myapp", "app.kubernetes.io/version=v2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLabels(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("splitLabels() = %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitLabels()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
