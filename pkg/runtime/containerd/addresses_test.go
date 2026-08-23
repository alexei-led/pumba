package containerd

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"strings"
	"testing"

	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testFibTrie = `Main:
  +-- 0.0.0.0/0 3 0 5
     +-- 10.42.0.0/24 2 0 2
        |-- 10.42.0.7
           /32 host LOCAL
     +-- 127.0.0.0/8 2 0 2
        |-- 127.0.0.1
           /32 host LOCAL
Local:
  +-- 10.42.0.7
     |-- 10.42.0.7
        /32 host LOCAL
`

const testIPv6Addresses = `00000000000000000000000000000001 01 80 10 80 lo
fd000000000000000000000000000007 02 40 00 80 eth0
fe800000000000000000000000000001 02 40 20 80 eth0
`

func TestParseProcessAddresses(t *testing.T) {
	addresses, err := parseProcessAddresses([]byte(testFibTrie), []byte(testIPv6Addresses))

	require.NoError(t, err)
	assert.Equal(t, []net.IP{
		net.ParseIP("10.42.0.7").To4(),
		net.ParseIP("fd00::7"),
	}, addresses)
}

func TestContainerAddresses(t *testing.T) {
	api := NewMockapiClient(t)
	cntr := NewMockContainer(t)
	task := NewMockTask(t)
	api.EXPECT().LoadContainer(mock.Anything, "target-id").Return(cntr, nil)
	cntr.EXPECT().Task(mock.Anything, mock.Anything).Return(task, nil)
	task.EXPECT().Pid().Return(uint32(1234))
	client := newTestClient(api)
	client.sameNetworkNamespace = func(uint32) (bool, error) { return false, nil }
	client.readProcessFile = func(path string) ([]byte, error) {
		switch {
		case strings.HasSuffix(path, "/net/fib_trie"):
			return []byte(testFibTrie), nil
		case strings.HasSuffix(path, "/net/if_inet6"):
			return []byte(testIPv6Addresses), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}

	addresses, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

	require.NoError(t, err)
	assert.Equal(t, []net.IP{net.ParseIP("10.42.0.7").To4(), net.ParseIP("fd00::7")}, addresses)
}

func TestContainerAddressesErrors(t *testing.T) {
	t.Run("load container", func(t *testing.T) {
		api := NewMockapiClient(t)
		api.EXPECT().LoadContainer(mock.Anything, "target-id").Return(nil, errors.New("load failed"))
		client := newTestClient(api)

		_, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

		require.ErrorContains(t, err, "load failed")
	})

	t.Run("get task", func(t *testing.T) {
		api := NewMockapiClient(t)
		cntr := NewMockContainer(t)
		api.EXPECT().LoadContainer(mock.Anything, "target-id").Return(cntr, nil)
		cntr.EXPECT().Task(mock.Anything, mock.Anything).Return(nil, errors.New("task failed"))
		client := newTestClient(api)

		_, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

		require.ErrorContains(t, err, "task failed")
	})

	t.Run("missing task PID", func(t *testing.T) {
		client := addressTestClient(t, 0)

		_, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

		require.ErrorContains(t, err, "has no PID")
	})

	t.Run("inspect namespace", func(t *testing.T) {
		client := addressTestClient(t, 1234)
		client.sameNetworkNamespace = func(uint32) (bool, error) {
			return false, errors.New("namespace failed")
		}

		_, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

		require.ErrorContains(t, err, "namespace failed")
	})

	for _, tc := range []struct {
		name       string
		failedFile string
	}{
		{name: "read IPv4", failedFile: "/net/fib_trie"},
		{name: "read IPv6", failedFile: "/net/if_inet6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := addressTestClient(t, 1234)
			client.sameNetworkNamespace = func(uint32) (bool, error) { return false, nil }
			client.readProcessFile = func(path string) ([]byte, error) {
				if strings.HasSuffix(path, tc.failedFile) {
					return nil, errors.New("read failed")
				}
				return []byte(testFibTrie), nil
			}

			_, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

			require.ErrorContains(t, err, "read failed")
		})
	}
}

func TestContainerAddressesAllowsMissingIPv6ProcFile(t *testing.T) {
	client := addressTestClient(t, 1234)
	client.sameNetworkNamespace = func(uint32) (bool, error) { return false, nil }
	client.readProcessFile = func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "/net/if_inet6") {
			return nil, fs.ErrNotExist
		}
		return []byte(testFibTrie), nil
	}

	addresses, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

	require.NoError(t, err)
	assert.Equal(t, []net.IP{net.ParseIP("10.42.0.7").To4()}, addresses)
}

func TestParseProcessAddressesRejectsInvalidIPv6(t *testing.T) {
	_, err := parseProcessAddresses(nil, []byte("not-hex 01 80 10 80 eth0"))

	require.ErrorContains(t, err, "invalid IPv6 address")
}

func TestContainerAddressesHostNetworkReturnsNoAddress(t *testing.T) {
	api := NewMockapiClient(t)
	cntr := NewMockContainer(t)
	task := NewMockTask(t)
	api.EXPECT().LoadContainer(mock.Anything, "target-id").Return(cntr, nil)
	cntr.EXPECT().Task(mock.Anything, mock.Anything).Return(task, nil)
	task.EXPECT().Pid().Return(uint32(1234))
	client := newTestClient(api)
	client.sameNetworkNamespace = func(uint32) (bool, error) { return true, nil }

	addresses, err := client.ContainerAddresses(context.Background(), &ctr.Container{ContainerID: "target-id"})

	require.NoError(t, err)
	assert.Empty(t, addresses)
}

func addressTestClient(t *testing.T, pid uint32) *containerdClient {
	t.Helper()
	api := NewMockapiClient(t)
	cntr := NewMockContainer(t)
	task := NewMockTask(t)
	api.EXPECT().LoadContainer(mock.Anything, "target-id").Return(cntr, nil)
	cntr.EXPECT().Task(mock.Anything, mock.Anything).Return(task, nil)
	task.EXPECT().Pid().Return(pid)
	return newTestClient(api)
}
