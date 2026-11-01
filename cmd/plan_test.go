package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/runtime/netns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestShellJoin(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{name: "plain words", argv: []string{"iptables", "-I", "INPUT", "--comment", "pumba:r1", "10.0.0.0/8"}, want: "iptables -I INPUT --comment pumba:r1 10.0.0.0/8"},
		{name: "spaces and newlines", argv: []string{"sh", "-ec", "tc qdisc show\nexit 0"}, want: "sh -ec 'tc qdisc show\nexit 0'"},
		{name: "single quotes", argv: []string{"sh", "-ec", "printf '%s'"}, want: `sh -ec 'printf '"'"'%s'"'"''`},
		{name: "empty argument", argv: []string{"echo", ""}, want: "echo ''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shellJoin(tt.argv))
		})
	}
}

func TestRenderPlan(t *testing.T) {
	tests := []struct {
		name   string
		target chaos.TargetRef
		plan   *chaos.Plan
		want   string
	}{
		{
			name:   "sidecar",
			target: chaos.TargetRef{ID: "a1b2c3d4e5f6a7b8", Name: "web"},
			plan: &chaos.Plan{
				Via: "sidecar", Image: "nettools",
				Apply: [][]string{
					{"iptables", "-I", "INPUT", "-s", "10.0.0.1/32", "-j", "DROP"},
					{"iptables", "-I", "INPUT", "-s", "10.0.0.2/32", "-j", "DROP"},
				},
				Revert: [][]string{
					{"iptables", "-D", "INPUT", "-s", "10.0.0.1/32", "-j", "DROP"},
					{"iptables", "-D", "INPUT", "-s", "10.0.0.2/32", "-j", "DROP"},
				},
			},
			want: "would run on web via sidecar nettools:\n" +
				"  apply:  iptables -I INPUT -s 10.0.0.1/32 -j DROP\n" +
				"  apply:  iptables -I INPUT -s 10.0.0.2/32 -j DROP\n" +
				"  revert: iptables -D INPUT -s 10.0.0.1/32 -j DROP\n" +
				"  revert: iptables -D INPUT -s 10.0.0.2/32 -j DROP\n",
		},
		{
			name:   "exec in an unnamed target",
			target: chaos.TargetRef{ID: "a1b2c3d4e5f6a7b8"},
			plan:   &chaos.Plan{Via: "exec", Apply: [][]string{{"sh", "-ec", "start"}}, Revert: [][]string{{"sh", "-ec", "stop"}}},
			want:   "would run on a1b2c3d4e5f6 via exec in the target:\n  apply:  sh -ec start\n  revert: sh -ec stop\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			require.NoError(t, renderPlan(&out, tt.target, tt.plan))
			assert.Equal(t, tt.want, out.String())
		})
	}
}

func TestPlanTextPrintsOnlyPlans(t *testing.T) {
	var out bytes.Buffer
	p := &planText{w: &out}
	target := chaos.TargetRef{ID: "id", Name: "web"}
	p.Emit(&chaos.Event{Type: chaos.EventTargetApplied, Applied: &chaos.TargetApplied{Target: target}})
	p.Emit(&chaos.Event{Type: chaos.EventTargetResult, Result: &chaos.TargetResult{Target: target, Status: chaos.TargetPlanned}})
	p.Emit(&chaos.Event{Type: chaos.EventRunEnd, End: &chaos.RunEnd{Status: chaos.RunOK}})
	assert.Empty(t, out.String())

	p.Emit(&chaos.Event{Type: chaos.EventTargetResult, Result: &chaos.TargetResult{Target: target, Status: chaos.TargetPlanned, Plan: &chaos.Plan{
		Via: "exec", Apply: [][]string{{"iptables", "-I", "INPUT"}}, Revert: [][]string{{"iptables", "-D", "INPUT"}},
	}}})
	assert.Equal(t, "would run on web via exec in the target:\n  apply:  iptables -I INPUT\n  revert: iptables -D INPUT\n", out.String())
}

// mockIPTablesDryRun plans with the runtimes' shared builder, so the CLI
// output carries the real argv.
func mockIPTablesDryRun(t *testing.T) *ctr.MockClient {
	t.Helper()
	web := &ctr.Container{ContainerID: "web-id", ContainerName: "web", State: ctr.StateRunning, Labels: map[string]string{}}
	client := ctr.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), ctr.ListOpts{}).Return([]*ctr.Container{web}, nil).Once()
	client.EXPECT().PlanIPTablesContainer(mock.AnythingOfType("*container.IPTablesRequest"), mock.AnythingOfType("*container.IPTablesRequest")).
		RunAndReturn(func(add, del *ctr.IPTablesRequest) (*ctr.CommandPlan, error) {
			return netns.IPTablesPlan(add, del), nil
		}).Once()
	client.EXPECT().Close().Return(nil).Maybe()
	return client
}

func TestRunDryRunPrintsPlan(t *testing.T) {
	code, stdout, _ := runPumba(t, mockIPTablesDryRun(t),
		"--dry-run", "--run-id", "r1", "iptables", "--iptables-image", "img", "--duration", "1h", "loss", "--probability", "0.2", "web")

	assert.Equal(t, 0, code)
	assert.Equal(t, "would run on web via sidecar img:\n"+
		"  apply:  iptables -I INPUT -i eth0 -m comment --comment pumba:r1 -m statistic --mode random --probability 0.20 -j DROP\n"+
		"  revert: iptables -D INPUT -i eth0 -m comment --comment pumba:r1 -m statistic --mode random --probability 0.20 -j DROP\n", stdout)
}

func TestRunDryRunNDJSONCarriesPlanOnly(t *testing.T) {
	code, stdout, _ := runPumba(t, mockIPTablesDryRun(t),
		"-o", "ndjson", "--dry-run", "--run-id", "r1", "iptables", "--iptables-image", "img", "--duration", "1h", "loss", "--probability", "0.2", "web")

	assert.Equal(t, 0, code)
	assert.NotContains(t, stdout, "would run")
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, 3)
	var result struct {
		Type   chaos.EventType    `json:"type"`
		Status chaos.TargetStatus `json:"status"`
		Plan   chaos.Plan         `json:"plan"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &result))
	assert.Equal(t, chaos.EventTargetResult, result.Type)
	assert.Equal(t, chaos.TargetPlanned, result.Status)
	assert.Equal(t, chaos.Plan{
		Via: "sidecar", Image: "img",
		Apply:  [][]string{{"iptables", "-I", "INPUT", "-i", "eth0", "-m", "comment", "--comment", "pumba:r1", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"}},
		Revert: [][]string{{"iptables", "-D", "INPUT", "-i", "eth0", "-m", "comment", "--comment", "pumba:r1", "-m", "statistic", "--mode", "random", "--probability", "0.20", "-j", "DROP"}},
	}, result.Plan)
}

func TestRunNetemDryRunPlans(t *testing.T) {
	for _, mode := range []string{"text", "ndjson"} {
		t.Run(mode, func(t *testing.T) {
			client := ctr.NewMockClient(t)
			web := &ctr.Container{ContainerID: "web-id", ContainerName: "web", State: ctr.StateRunning}
			client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), ctr.ListOpts{}).Return([]*ctr.Container{web}, nil).Once()
			client.EXPECT().PlanNetemContainer(&ctr.NetemRequest{
				Container: web, Interface: "eth0", Command: []string{"delay", "100ms"}, IPs: []*net.IPNet{},
				Duration: time.Hour, DryRun: true, RunID: "r1", Sidecar: ctr.SidecarSpec{Pull: true},
			}).RunAndReturn(func(req *ctr.NetemRequest) (*ctr.CommandPlan, error) { return netns.NetemPlan(req), nil }).Once()
			client.EXPECT().Close().Return(nil).Maybe()
			code, stdout, _ := runPumba(t, client, "--dry-run", "--run-id", "r1", "-o", mode,
				"netem", "--duration", "1h", "--tc-image", "", "delay", "--time", "100", "--jitter", "0", "--correlation", "0", "web")
			require.Equal(t, 0, code)
			if mode == "text" {
				assert.Contains(t, stdout, "would run on web via exec in the target:")
				assert.Contains(t, stdout, "apply:  sh -ec ")
				assert.Contains(t, stdout, "revert: sh -ec ")
				return
			}
			assert.Equal(t, []chaos.EventType{chaos.EventRunStart, chaos.EventTargetResult, chaos.EventRunEnd}, types(parseNDJSON(t, stdout)))
			var result struct {
				Status  chaos.TargetStatus `json:"status"`
				Cleanup *chaos.Cleanup     `json:"cleanup"`
				Plan    *chaos.Plan        `json:"plan"`
			}
			require.NoError(t, json.Unmarshal([]byte(strings.Split(stdout, "\n")[1]), &result))
			assert.Equal(t, chaos.TargetPlanned, result.Status)
			assert.Nil(t, result.Cleanup)
			require.NotNil(t, result.Plan)
			assert.Equal(t, "exec", result.Plan.Via)
			assert.Empty(t, result.Plan.Image)
			assert.Equal(t, []string{"sh", "-ec"}, result.Plan.Apply[0][:2])
			assert.Equal(t, []string{"sh", "-ec"}, result.Plan.Revert[0][:2])
		})
	}
}

func TestRunDryRunPlanSelectionGuards(t *testing.T) {
	for _, action := range [][]string{
		{"netem", "--duration", "1h", "delay", "--time", "100"},
		{"iptables", "--duration", "1h", "loss", "--probability", "0.2"},
	} {
		for _, count := range []int{0, 2} {
			t.Run(fmt.Sprintf("%s/%d targets", action[0], count), func(t *testing.T) {
				client := ctr.NewMockClient(t)
				targets := []*ctr.Container{}
				for i := range count {
					targets = append(targets, &ctr.Container{ContainerID: fmt.Sprintf("web-%d", i)})
				}
				client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), ctr.ListOpts{}).Return(targets, nil).Once()
				client.EXPECT().Close().Return(nil).Maybe()
				args := append([]string{"--dry-run", "-o", "ndjson", "--max-targets", "1"}, action...)
				code, stdout, _ := runPumba(t, client, append(args, "re2:^web-")...)
				lines := parseNDJSON(t, stdout)
				assert.Equal(t, []chaos.EventType{chaos.EventRunStart, chaos.EventRunEnd}, types(lines))
				if count == 0 {
					assert.Equal(t, 0, code)
					assert.Equal(t, string(chaos.RunNoTargets), lines[1].Status)
				} else {
					assert.Equal(t, 1, code)
					assert.Equal(t, string(chaos.RunRefused), lines[1].Status)
				}
			})
		}
	}
}

func TestRunDryRunPlanFailure(t *testing.T) {
	client := ctr.NewMockClient(t)
	web := &ctr.Container{ContainerID: "web-id", ContainerName: "web", State: ctr.StateRunning}
	client.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), ctr.ListOpts{}).Return([]*ctr.Container{web}, nil).Once()
	client.EXPECT().PlanNetemContainer(mock.AnythingOfType("*container.NetemRequest")).Return(nil, errors.New("planning unavailable")).Once()
	client.EXPECT().Close().Return(nil).Maybe()
	code, stdout, _ := runPumba(t, client, "--dry-run", "-o", "ndjson", "netem", "--duration", "1h", "delay", "--time", "100", "web")
	assert.Equal(t, 1, code)
	lines := parseNDJSON(t, stdout)
	assert.Equal(t, []chaos.EventType{chaos.EventRunStart, chaos.EventTargetResult, chaos.EventRunEnd}, types(lines))
	assert.Equal(t, "failed", lines[1].Status)
	assert.Equal(t, chaos.CodeCommandFailed, lines[1].Error.Code)
	assert.NotContains(t, stdout, `"plan":`)
}
