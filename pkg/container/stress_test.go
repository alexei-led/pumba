package container

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	"github.com/docker/docker/api/types/container"

	"github.com/stretchr/testify/mock"
)

// MockConn implements the needed methods for a mock connection
type MockStressConn struct {
	mock.Mock
}

func (m *MockStressConn) Read(b []byte) (n int, err error) {
	return 0, io.EOF
}

func (m *MockStressConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *MockStressConn) Close() error {
	return nil
}

func (m *MockStressConn) LocalAddr() net.Addr {
	return nil
}

func (m *MockStressConn) RemoteAddr() net.Addr {
	return nil
}

func (m *MockStressConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockStressConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockStressConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// Test for StressContainer functionality - Testing only dry run mode and error cases
func TestStressContainerBasic(t *testing.T) {
	type args struct {
		ctx       context.Context
		c         *Container
		stressors []string
		image     string
		pull      bool
		duration  time.Duration
		dryrun    bool
	}
	tests := []struct {
		name    string
		args    args
		mockSet func(*mocks.APIClient, *Container, []string, string, bool, time.Duration, bool)
		wantErr bool
	}{
		{
			name: "stress container dry run",
			args: args{
				ctx:       context.TODO(),
				c:         &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123", "Name", "test-container"))},
				stressors: []string{"--cpu", "2", "--timeout", "30s"},
				image:     "alexeiled/stress-ng:latest",
				duration:  30 * time.Second,
				dryrun:    true,
			},
			mockSet: func(api *mocks.APIClient, c *Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) {
				// No mocks needed for dry run
			},
			wantErr: false,
		},
		{
			name: "stress container image pull failure",
			args: args{
				ctx:       context.TODO(),
				c:         &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123", "Name", "test-container"))},
				stressors: []string{"--cpu", "2", "--timeout", "30s"},
				image:     "alexeiled/stress-ng:latest",
				pull:      true,
				duration:  30 * time.Second,
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, c *Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) {
				api.On("ImagePull", mock.Anything, image, mock.Anything).Return(nil, errors.New("pull error")).Once()
			},
			wantErr: true,
		},
		{
			name: "stress container creation failure",
			args: args{
				ctx:       context.TODO(),
				c:         &Container{ContainerInfo: DetailsResponse(AsMap("ID", "abc123", "Name", "test-container"))},
				stressors: []string{"--cpu", "2", "--timeout", "30s"},
				image:     "alexeiled/stress-ng:latest",
				pull:      false,
				duration:  30 * time.Second,
				dryrun:    false,
			},
			mockSet: func(api *mocks.APIClient, c *Container, stressors []string, image string, pull bool, duration time.Duration, dryrun bool) {
				api.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{}, errors.New("create error")).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewMockEngine()
			tt.mockSet(api, tt.args.c, tt.args.stressors, tt.args.image, tt.args.pull, tt.args.duration, tt.args.dryrun)

			client := dockerClient{containerAPI: api, imageAPI: api}
			_, _, _, err := client.StressContainer(tt.args.ctx, tt.args.c, tt.args.stressors, tt.args.image, tt.args.pull, tt.args.duration, tt.args.dryrun)

			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.StressContainer() error = %v, wantErr %v", err, tt.wantErr)
			}
			api.AssertExpectations(t)
		})
	}
}
