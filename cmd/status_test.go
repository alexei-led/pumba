package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRenderStatus(t *testing.T) {
	x := "X"
	rep := &chaos.StatusReport{
		Check: true,
		Targets: []chaos.StatusTarget{
			{Target: chaos.TargetRef{ID: "a", Name: "api"}, State: &chaos.TargetState{}},
			{Target: chaos.TargetRef{ID: "d", Name: "db"}, Error: "inspecting db: tc: not found", State: &chaos.TargetState{}},
			{Target: chaos.TargetRef{ID: "w", Name: "web"}, State: &chaos.TargetState{
				Rules:    []chaos.StateRule{{RunID: "X", Chain: "INPUT", Spec: "-A INPUT -m comment --comment pumba:X -j DROP"}},
				Qdiscs:   []chaos.StateQdisc{{Device: "eth0", Kind: "netem", Ownership: "pumba-owned"}},
				Sidecars: []chaos.StateSidecar{{ID: "s1", Name: "pumba-s1", Kind: "netem", State: "exited", TargetID: "w", RunID: &x}},
			}},
		},
		Coverage: &chaos.StatusCoverage{
			Verdict:        chaos.VerdictDirty,
			Reasons:        []string{"web: iptables rule of run X"},
			OrphanSidecars: []chaos.StateSidecar{{ID: "s2", Name: "pumba-s2", Kind: "stress", State: "running", TargetID: "0123456789abcdef"}},
			MissingTargets: []string{"ghost"},
			ListingGaps:    []chaos.ListingGap{{ID: "broken", Error: "no task"}},
		},
	}
	var out bytes.Buffer

	require.NoError(t, renderStatus(&out, rep))

	assert.Equal(t, strings.Join([]string{
		"TARGET        KIND        DETAIL                                         RUN",
		"api           -           clean                                          -",
		"db            error       inspecting db: tc: not found                   -",
		"web           iptables    -A INPUT -m comment --comment pumba:X -j DROP  X",
		"web           qdisc       eth0 netem (pumba-owned)                       -",
		"web           sidecar     netem pumba-s1 (exited)                        X",
		"0123456789ab  sidecar     orphan stress pumba-s2 (running)               -",
		"ghost         missing     not found or not running                       -",
		"broken        unreadable  no task                                        -",
		"verdict: dirty (web: iptables rule of run X)",
		"",
	}, "\n"), out.String())
}

func TestRenderStatusWithoutCheckHasNoVerdict(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, renderStatus(&out, &chaos.StatusReport{Coverage: &chaos.StatusCoverage{Verdict: chaos.VerdictUnknown}}))
	assert.Equal(t, "TARGET  KIND  DETAIL  RUN\n", out.String())
}

// mockStatus serves one running target "web" and inspects it with fs.
func mockStatus(t *testing.T, fs *ctr.FaultState) *ctr.MockClient {
	t.Helper()
	web := &ctr.Container{ContainerID: "web-id", ContainerName: "web", State: ctr.StateRunning, Labels: map[string]string{}}
	client := ctr.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).
		RunAndReturn(func(_ context.Context, fn ctr.FilterFunc, _ ctr.ListOpts) ([]*ctr.Container, error) {
			if fn(web) {
				return []*ctr.Container{web}, nil
			}
			return nil, nil
		}).Times(2)
	client.EXPECT().InspectNetns(mock.Anything, &ctr.InspectRequest{Container: web, Sidecar: ctr.SidecarSpec{Image: "img", Pull: true}, RunID: "r1"}).
		Return(fs, nil).Once()
	client.EXPECT().Close().Return(nil).Maybe()
	return client
}

var taggedRule = &ctr.FaultState{Rules: []ctr.PumbaRule{{RunID: "X", Chain: "INPUT", Spec: "-A INPUT -j DROP"}}}

func TestRunStatusCheckText(t *testing.T) {
	tests := []struct {
		name   string
		fs     *ctr.FaultState
		code   int
		stdout string
	}{
		{
			name:   "clean",
			fs:     &ctr.FaultState{},
			stdout: "TARGET  KIND  DETAIL  RUN\nweb     -     clean   -\nverdict: clean\n",
		},
		{
			name:   "dirty",
			fs:     taggedRule,
			code:   1,
			stdout: "TARGET  KIND      DETAIL            RUN\nweb     iptables  -A INPUT -j DROP  X\nverdict: dirty (web: iptables rule of run X)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, _ := runPumba(t, mockStatus(t, tt.fs), "--run-id", "r1", "status", "--check", "--tc-image", "img", "web")
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.stdout, stdout)
		})
	}
}

func TestRunStatusNDJSONCarriesStateAndCoverage(t *testing.T) {
	code, stdout, _ := runPumba(t, mockStatus(t, taggedRule), "-o", "ndjson", "--run-id", "r1", "status", "--check", "--run-id", "X", "--tc-image", "img")

	assert.Equal(t, 1, code)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, 3)
	var result struct {
		Type  chaos.EventType   `json:"type"`
		State chaos.TargetState `json:"state"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &result))
	assert.Equal(t, chaos.EventTargetResult, result.Type)
	assert.Equal(t, []chaos.StateRule{{RunID: "X", Chain: "INPUT", Spec: "-A INPUT -j DROP"}}, result.State.Rules)
	var end struct {
		Type     chaos.EventType      `json:"type"`
		RunID    string               `json:"run_id"`
		Status   chaos.RunStatus      `json:"status"`
		Coverage chaos.StatusCoverage `json:"coverage"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[2]), &end))
	assert.Equal(t, chaos.EventRunEnd, end.Type)
	assert.Equal(t, "r1", end.RunID, "the global --run-id names the invocation; status --run-id only filters")
	assert.Equal(t, chaos.RunDirty, end.Status)
	assert.Equal(t, chaos.VerdictDirty, end.Coverage.Verdict)
	assert.Equal(t, "X", end.Coverage.RunFilter)
}

func TestRunStatusCheckRejectsInterval(t *testing.T) {
	client := ctr.NewMockClient(t)
	client.EXPECT().Close().Return(nil).Maybe()

	code, stdout, _ := runPumba(t, client, "-o", "ndjson", "--interval", "1s", "status", "--check")

	assert.Equal(t, 1, code)
	lines := parseNDJSON(t, stdout)
	require.Len(t, lines, 1)
	assert.Equal(t, string(chaos.RunUsageError), lines[0].Status)
	assert.Equal(t, chaos.CodeInvalidArgument, lines[0].Error.Code)
}

func TestRunStatusCheckFailsOnListingErrorDespiteSkipError(t *testing.T) {
	client := ctr.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).
		Return(nil, errors.New("socket closed")).Once()
	client.EXPECT().Close().Return(nil).Maybe()

	code, stdout, _ := runPumba(t, client, "--skip-error", "status", "--check", "web")

	assert.Equal(t, 1, code)
	assert.Equal(t, "TARGET  KIND  DETAIL  RUN\nverdict: unknown (listing failed: listing pumba sidecars: socket closed)\n", stdout)
}
