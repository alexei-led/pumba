package chaos_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunID(t *testing.T) {
	a, b := chaos.NewRunID(), chaos.NewRunID()
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{12}$`), a)
	assert.NotEqual(t, a, b)
	assert.NoError(t, chaos.ValidateRunID(a))
}

func TestResolveRunID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr bool
	}{
		{name: "user value kept", id: "exp-42_v1.2", want: "exp-42_v1.2"},
		{name: "single char", id: "a", want: "a"},
		{name: "max length", id: strings.Repeat("a", 48), want: strings.Repeat("a", 48)},
		{name: "too long", id: strings.Repeat("a", 49), wantErr: true},
		{name: "space", id: "my run", wantErr: true},
		{name: "colon", id: "pumba:1", wantErr: true},
		{name: "quote", id: `a"b`, wantErr: true},
		{name: "slash", id: "a/b", wantErr: true},
		{name: "newline", id: "a\nb", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chaos.ResolveRunID(tt.id)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid --run-id")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveRunIDGeneratesWhenEmpty(t *testing.T) {
	got, err := chaos.ResolveRunID("")
	require.NoError(t, err)
	assert.Len(t, got, 12)
}
