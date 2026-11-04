package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRevertUsageConstraints(t *testing.T) {
	for _, args := range [][]string{
		{"revert"},
		{"--run-id", "old", "revert", "--run-id", "old"},
		{"--random", "revert", "--run-id", "old"},
		{"--skip-error", "revert", "--run-id", "old"},
		{"--interval", "1s", "revert", "--run-id", "old"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			client := ctr.NewMockClient(t)
			client.EXPECT().Close().Return(nil).Maybe()
			code, stdout, _ := runPumba(t, client, append([]string{"-o", "ndjson"}, args...)...)
			require.Equal(t, 1, code)
			require.Contains(t, stdout, `"status":"usage_error"`)
			require.NotContains(t, stdout, `"type":"target.applied"`)
		})
	}
}

func TestRevertDryRunCLIPlan(t *testing.T) {
	for _, mode := range []string{"text", "ndjson"} {
		t.Run(mode, func(t *testing.T) {
			client := ctr.NewMockClient(t)
			client.EXPECT().Close().Return(nil).Maybe()
			target := &ctr.Container{ContainerID: "web", ContainerName: "web", State: ctr.StateRunning}
			rule := ctr.PumbaRule{Backend: "iptables-legacy", RunID: "old", Chain: "INPUT", Spec: "-A INPUT -m comment --comment pumba:old -j DROP"}
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(o ctr.ListOpts) bool { return !o.All && o.OnSkip != nil })).Return([]*ctr.Container{target}, nil).Once()
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(o ctr.ListOpts) bool { return o.All && o.OnSkip != nil })).Return(nil, nil).Once()
			client.EXPECT().InspectNetns(mock.Anything, &ctr.InspectRequest{Container: target, RunID: "new"}).Return(&ctr.FaultState{Rules: []ctr.PumbaRule{rule}}, nil).Once()
			req := &ctr.RuleRecoveryRequest{Container: target, RunFilter: "old", InvocationID: "new", Rules: []ctr.PumbaRule{rule}}
			client.EXPECT().PlanRecoveryRules(req).RunAndReturn(netns.RecoveryPlan).Once()
			code, stdout, _ := runPumba(t, client, "-o", mode, "--dry-run", "--run-id", "new", "revert", "--run-id", "old", "--tc-image", "", "--pull-image=false", "web")
			require.Zero(t, code)
			if mode == "text" {
				require.Contains(t, stdout, "would run on web")
				require.Contains(t, stdout, "iptables-legacy -t filter -D INPUT")
				return
			}
			var events []map[string]any
			for line := range strings.Lines(stdout) {
				var event map[string]any
				require.NoError(t, json.Unmarshal([]byte(line), &event))
				events = append(events, event)
			}
			require.Len(t, events, 3)
			require.Equal(t, "new", events[0]["run_id"])
			require.Equal(t, "old", events[0]["params"].(map[string]any)["run-id"])
			require.Equal(t, string(chaos.EventTargetResult), events[1]["type"])
			require.Equal(t, "planned", events[1]["status"])
			require.Contains(t, events[1], "recovery")
			require.Equal(t, "ok", events[2]["status"])
			require.Equal(t, float64(0), events[2]["artifact_counts"].(map[string]any)["selected"])
		})
	}
}
