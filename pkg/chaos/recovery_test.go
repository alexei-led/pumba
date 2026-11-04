package chaos

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

func TestArtifactResultEncodingAndCounts(t *testing.T) {
	recorder := NewRecorder(NopSink{}, "test")
	recorder.Emit(&Event{Type: EventRunStart, Action: "revert", RunID: "new", Start: &RunStart{}})
	identity := container.SidecarIdentity{ID: "sidecar", TargetID: "missing", RunID: "old", Kind: container.SidecarKindStress}
	event := &Event{Schema: EventSchema, Type: EventArtifactResult, RunID: "new", Artifact: &ArtifactResult{Artifact: identity, Status: TargetOK}}
	recorder.Emit(event)
	raw, err := json.Marshal(event)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, "artifact.result", decoded["type"])
	require.NotContains(t, decoded, "target")
	require.Equal(t, "missing", decoded["artifact"].(map[string]any)["target_id"])
	end, out := recorder.RunEnd(nil, false)
	require.Equal(t, RunOK, out.Status)
	require.Zero(t, end.End.Counts.Selected)
	require.Equal(t, 1, end.End.ArtifactCounts.OK)
	recorder.Emit(&Event{Type: EventArtifactResult, Artifact: &ArtifactResult{Artifact: identity, Status: TargetCleanupFailed}})
	end, out = recorder.RunEnd(errors.New("delete denied"), false)
	require.Equal(t, RunCleanupFailed, out.Status)
	require.Equal(t, 1, out.ExitCode)
	require.Equal(t, 1, end.End.ArtifactCounts.CleanupFailed)
}
