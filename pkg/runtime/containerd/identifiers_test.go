package containerd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Exercise the real allocation sites in fresh processes: concurrent calls in
// one process alone cannot catch identifiers that reset at process startup.
func TestContainerdIDsAcrossProcesses(t *testing.T) {
	if os.Getenv("PUMBA_TEST_ID_CHILD") == "1" {
		collectContainerdIDs(t)
		return
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	var mu sync.Mutex
	seen := make(map[string]bool)
	for i := range 4 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestContainerdIDsAcrossProcesses$")
			cmd.Env = append(os.Environ(), "PUMBA_TEST_ID_CHILD=1")
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", out)
			var count int
			for line := range strings.SplitSeq(string(out), "\n") {
				if id, ok := strings.CutPrefix(line, "PUMBA_TEST_ID:"); ok {
					mu.Lock()
					assert.False(t, seen[id], "identifier %q collided across fresh processes", id)
					seen[id] = true
					mu.Unlock()
					count++
				}
			}
			require.Equal(t, 4, count, "must exercise target exec, sidecar, sidecar exec and stress identifiers")
		})
	}
}

func collectContainerdIDs(t *testing.T) {
	t.Helper()
	api := NewMockapiClient(t)
	task := newRunningTask(t)
	setupExec(task, newSuccessProcess(t))
	setupLoadContainer(api, "c1", newMockContainer(t, "c1", "nginx", nil, task))
	require.NoError(t, newTestClient(api).ExecContainer(context.Background(), testContainer("c1"), "true", nil, false))
	fmt.Println("PUMBA_TEST_ID:" + recordedOperationID(t, task.Calls, "Exec"))

	api, task = setupNetSidecar(t, ctr.SidecarLabels("", "c1", ctr.SidecarKindNetem))
	setupExec(task, newSuccessProcess(t))
	require.NoError(t, newTestClient(api).NetemContainer(context.Background(), &ctr.NetemRequest{
		Container: testContainer("c1"), Interface: "eth0", Command: []string{"delay", "100ms"}, Sidecar: ctr.SidecarSpec{Image: "nettools"},
	}))
	fmt.Println("PUMBA_TEST_ID:" + recordedOperationID(t, api.Calls, "NewContainer"))
	fmt.Println("PUMBA_TEST_ID:" + recordedOperationID(t, task.Calls, "Exec"))

	api = setupStressTarget(t, 42, "0::/system.slice/docker-c1.scope\n")
	api.EXPECT().GetImage(mock.Anything, "stress-ng").Return(NewMockImage(t), nil)
	api.EXPECT().NewContainer(mock.Anything, stressPrefix(), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()
	_, err := newTestClient(api).StressContainer(context.Background(), stressReq(testContainer("c1"), []string{"--cpu", "1"}, "stress-ng", false, time.Second, false, false))
	require.ErrorIs(t, err, assert.AnError)
	fmt.Println("PUMBA_TEST_ID:" + recordedOperationID(t, api.Calls, "NewContainer"))
}

func recordedOperationID(t *testing.T, calls []mock.Call, method string) string {
	t.Helper()
	for _, call := range calls {
		if call.Method == method {
			return call.Arguments.String(1)
		}
	}
	t.Fatalf("missing %s call", method)
	return ""
}
