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
		{name: "cleanup failure", err: errors.Join(refused, errors.New("cleanup failed"))},
		{name: "read failure", err: errors.Join(refused, errors.New("incomplete output"))},
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
