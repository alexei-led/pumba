package container

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSidecarIdentityValidate(t *testing.T) {
	valid := SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: SidecarKindStress}
	cases := []struct {
		name string
		edit func(*SidecarIdentity)
		want string
	}{
		{name: "valid"},
		{name: "missing id", edit: func(i *SidecarIdentity) { i.ID = "" }, want: "invalid"},
		{name: "target cannot be artifact", edit: func(i *SidecarIdentity) { i.TargetID = i.ID }, want: "invalid"},
		{name: "missing run", edit: func(i *SidecarIdentity) { i.RunID = "" }, want: "invalid"},
		{name: "path id", edit: func(i *SidecarIdentity) { i.ID = "../artifact" }, want: "invalid"},
		{name: "inspect is not removable", edit: func(i *SidecarIdentity) { i.Kind = SidecarKindInspect }, want: "ineligible"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := valid
			if tc.edit != nil {
				tc.edit(&identity)
			}
			err := identity.Validate()
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}

func TestSidecarIdentityMatchesRequiresExactOwnership(t *testing.T) {
	identity := SidecarIdentity{ID: "sidecar", TargetID: "target", RunID: "run", Kind: SidecarKindNetem}
	labels := SidecarLabels(identity.RunID, identity.TargetID, identity.Kind)
	require.NoError(t, identity.Matches(identity.ID, labels))

	cases := []struct {
		name string
		id   string
		key  string
	}{
		{name: "different artifact", id: "other"},
		{name: "not marked sidecar", id: identity.ID, key: SkipLabel},
		{name: "different target", id: identity.ID, key: TargetLabel},
		{name: "different run", id: identity.ID, key: RunIDLabel},
		{name: "different kind", id: identity.ID, key: KindLabel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := make(map[string]string, len(labels))
			for key, value := range labels {
				changed[key] = value
			}
			if tc.key != "" {
				changed[tc.key] = "changed"
			}
			err := identity.Matches(tc.id, changed)
			require.ErrorContains(t, err, "identity or ownership changed")
		})
	}
}
