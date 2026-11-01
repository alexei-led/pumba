package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSidecarLabels(t *testing.T) {
	tests := []struct {
		name  string
		runID string
		want  map[string]string
	}{
		{
			name:  "tagged",
			runID: "run1",
			want: map[string]string{
				"com.gaiaadm.pumba.skip":   "true",
				"com.gaiaadm.pumba.run-id": "run1",
				"com.gaiaadm.pumba.target": "abc",
				"com.gaiaadm.pumba.kind":   "netem",
			},
		},
		{
			name: "untagged",
			want: map[string]string{
				"com.gaiaadm.pumba.skip":   "true",
				"com.gaiaadm.pumba.target": "abc",
				"com.gaiaadm.pumba.kind":   "netem",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := SidecarLabels(tt.runID, "abc", SidecarKindNetem)
			assert.Equal(t, tt.want, labels)
			assert.True(t, (&Container{Labels: labels}).IsPumbaSkip())
		})
	}
}
