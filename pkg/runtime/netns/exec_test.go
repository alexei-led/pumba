package netns

import (
	"errors"
	"fmt"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/alexei-led/pumba/pkg/tc"
	"github.com/stretchr/testify/require"
)

func TestNetemApplyErrorRequiresOnlyObservedStartupRefusal(t *testing.T) {
	argv, err := NetemStart(&ctr.NetemRequest{Interface: "eth0", Command: []string{"delay", "100ms"}})
	require.NoError(t, err)
	// Runtime error text includes the full script, hence the refusal branch
	// and its exit code even when the actual failure happened after mutation.
	message := fmt.Errorf("exec %v failed: refusing to replace a foreign or stale root qdisc; exit %d", argv, tc.NetemRefusedExitCode)
	refused := &ExecExitError{Code: tc.NetemRefusedExitCode, Argv: argv, Message: message.Error()}
	for _, tt := range []struct {
		name    string
		err     error
		refused bool
	}{
		{name: "success"},
		{name: "observed", err: refused, refused: true},
		{name: "wrapped", err: fmt.Errorf("command failed: %w", refused), refused: true},
		{name: "successful sidecar cleanup", err: errors.Join(refused, nil), refused: true},
		{name: "full script text alone", err: message},
		{name: "partial failure contains refusal branch", err: &ExecExitError{Code: 1, Argv: argv, Message: message.Error()}},
		{name: "probe status is not apply status", err: &ExecExitError{Code: tc.NetemRefusedExitCode, Argv: []string{"which", "sh"}, Message: message.Error()}},
		{name: "undifferentiated cleanup failure", err: errors.Join(refused, errors.New("cleanup failed"))},
		{name: "contradictory execution status", err: errors.Join(refused, &ExecExitError{Code: 1, Argv: argv, Message: message.Error()})},
		{name: "unrelated execution status", err: errors.Join(refused, &ExecExitError{Code: tc.NetemRefusedExitCode, Argv: []string{"which", "sh"}, Message: message.Error()})},
		{name: "read failure", err: errors.Join(refused, errors.New("incomplete output"))},
		{name: "observed refusal then bookkeeping failure", err: JoinExecCleanup(refused, errors.New("delete failed")), refused: true},
		{name: "bookkeeping failure after success", err: JoinExecCleanup(nil, errors.New("delete failed"))},
		{name: "incomplete execution then bookkeeping failure", err: JoinExecCleanup(errors.Join(refused, errors.New("incomplete output")), errors.New("delete failed"))},
		{name: "bookkeeping plus unknown execution", err: errors.Join(JoinExecCleanup(refused, errors.New("delete failed")), errors.New("unknown execution"))},
		{name: "wrapped unknown failure", err: fmt.Errorf("command: %w", errors.Join(refused, errors.New("unknown")))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := NetemApplyError(argv, tt.err)
			require.Equal(t, tt.refused, errors.Is(got, ctr.ErrNetemNotApplied))
			if tt.err == nil {
				require.NoError(t, got)
			} else {
				require.ErrorIs(t, got, tt.err)
			}
		})
	}
}
