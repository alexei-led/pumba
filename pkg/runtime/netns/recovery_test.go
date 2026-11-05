package netns

import (
	"context"
	"errors"
	"strings"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/require"
)

func recoveryRule(backend string) ctr.PumbaRule {
	return ctr.PumbaRule{Backend: backend, RunID: "old", Chain: "INPUT", Spec: "-A INPUT -m comment --comment pumba:old -j DROP"}
}

func TestRecoveryPlanContract(t *testing.T) {
	rule := recoveryRule("iptables-legacy")
	req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "t"}, Sidecar: ctr.SidecarSpec{Image: "nettools"}, RunFilter: "old", Rules: []ctr.PumbaRule{rule, rule}}
	plan, err := RecoveryPlan(req)
	require.NoError(t, err)
	require.Equal(t, &ctr.CommandPlan{Via: ctr.ViaSidecar, Image: "nettools", Revert: [][]string{
		{"iptables-legacy", "-t", "filter", "-D", "INPUT", "-m", "comment", "--comment", "pumba:old", "-j", "DROP"},
		{"iptables-legacy", "-t", "filter", "-D", "INPUT", "-m", "comment", "--comment", "pumba:old", "-j", "DROP"},
	}}, plan)
	req.Rules = make([]ctr.PumbaRule, maxRecoveryRules+1)
	_, err = RecoveryPlan(req)
	require.ErrorContains(t, err, "occurrences")
	rule.Spec += " -s " + strings.Repeat("a", maxRecoveryBytes)
	req.Rules = []ctr.PumbaRule{rule}
	_, err = RecoveryPlan(req)
	require.ErrorContains(t, err, "bytes")
}

func TestRecoverRulesSnapshots(t *testing.T) {
	for _, tt := range []struct {
		name                                         string
		authorized, present                          int
		deleteError, noDelete, verifyFailure, cancel bool
		wantDeletes                                  int
		wantErr                                      string
	}{
		{name: "duplicates", authorized: 2, present: 2, wantDeletes: 2},
		{name: "shared namespace already absent", authorized: 2},
		{name: "concurrent removal verified", authorized: 1, present: 1, deleteError: true, wantDeletes: 1},
		{name: "changed inventory", authorized: 1, present: 2, wantErr: "inventory changed"},
		{name: "failed deletion", authorized: 1, present: 1, noDelete: true, wantDeletes: 1, wantErr: "still present"},
		{name: "unreadable verification", authorized: 1, present: 1, verifyFailure: true, wantDeletes: 1, wantErr: "verification"},
		{name: "canceled after admitted delete", authorized: 2, present: 2, cancel: true, wantDeletes: 1, wantErr: "canceled"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rule := recoveryRule("iptables-nft")
			req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "t"}, RunFilter: "old"}
			for range tt.authorized {
				req.Rules = append(req.Rules, rule)
			}
			present, deletes := tt.present, 0
			err := RecoverRules(ctx, req, func(op context.Context, argv []string) (string, string, error) {
				require.NoError(t, op.Err())
				switch strings.Join(argv, " ") {
				case "tc qdisc show":
					if deletes > 0 && tt.verifyFailure {
						return "", "", errors.New("cannot inspect")
					}
					return "", "", nil
				case "iptables-nft -S":
					return strings.Repeat(rule.Spec+"\n", present), "", nil
				case "iptables --version":
					return "iptables v1.8.13 (nf_tables)", "", nil
				default:
					require.Equal(t, []string{"iptables-nft", "-t", "filter", "-D", "INPUT", "-m", "comment", "--comment", "pumba:old", "-j", "DROP"}, argv)
					_, bounded := op.Deadline()
					require.True(t, bounded)
					deletes++
					if !tt.noDelete {
						present--
					}
					if tt.cancel {
						cancel()
						require.NoError(t, op.Err())
					}
					if tt.deleteError {
						return "", "", errors.New("already deleted")
					}
					return "", "", nil
				}
			})
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			require.Equal(t, tt.wantDeletes, deletes)
		})
	}
}

func TestRecoverRulesRejectsBeforeIO(t *testing.T) {
	req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "t"}, RunFilter: "old", Rules: []ctr.PumbaRule{recoveryRule("iptables")}}
	err := RecoverRules(context.Background(), req, func(context.Context, []string) (string, string, error) {
		t.Fatal("must not inspect or mutate an invalid plan")
		return "", "", nil
	})
	require.Error(t, err)
}

func TestRecoveryKeepsIdenticalRulesInIndependentBackends(t *testing.T) {
	nft, legacy := recoveryRule("iptables-nft"), recoveryRule("iptables-legacy")
	remaining := map[string]bool{"iptables-nft": true, "iptables-legacy": true}
	req := &ctr.RuleRecoveryRequest{Container: &ctr.Container{ContainerID: "t"}, RunFilter: "old", Rules: []ctr.PumbaRule{nft, legacy}}
	var deleted []string
	err := RecoverRules(context.Background(), req, func(_ context.Context, argv []string) (string, string, error) {
		switch {
		case argv[0] == "tc":
			return "", "", nil
		case argv[0] == "iptables":
			require.Equal(t, []string{"iptables", "--version"}, argv, "never list through the mutable default alias")
			return "iptables v1.8.13 (legacy)", "", nil
		case len(argv) == 2 && argv[1] == "-S":
			if remaining[argv[0]] {
				return nft.Spec, "", nil
			}
			return "", "", nil
		default:
			require.Equal(t, []string{"-t", "filter", "-D", "INPUT", "-m", "comment", "--comment", "pumba:old", "-j", "DROP"}, argv[1:])
			remaining[argv[0]] = false
			deleted = append(deleted, argv[0])
			return "", "", nil
		}
	})
	require.NoError(t, err)
	require.Equal(t, []string{"iptables-nft", "iptables-legacy"}, deleted)
}
