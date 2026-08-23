package containerd

import (
	"context"
	"errors"
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
