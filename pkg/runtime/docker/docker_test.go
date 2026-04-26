package docker

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	ctr "github.com/alexei-led/pumba/pkg/container"
	"github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// fakeExecAttach returns a HijackedResponse suitable for mocking
// ContainerExecAttach in tests that don't care about the exec stream output.
func fakeExecAttach() types.HijackedResponse {
	conn := &mockConn{}
	conn.On("Close").Return(nil)
	return types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(strings.NewReader("")),
	}
}

func NewMockEngine() *mocks.APIClient {
	return new(mocks.APIClient)
}

func mockAllContainers(_ *ctr.Container) bool {
	return true
}
func mockNoContainers(_ *ctr.Container) bool {
	return false
}

func Test_dockerClient_stressContainerCommand(t *testing.T) {
	type args struct {
		ctx       context.Context
		targetID  string
		stressors []string
		image     string
		pull      bool
	}
	tests := []struct {
		name       string
		args       args
		mockInit   func(context.Context, *mocks.APIClient, *mockConn, string, []string, string, bool)
		want       string
		wantOutput string
		wantErr    bool
		wantErrCh  bool
	}{
		{
			name: "stress test with pull image",
			args: args{
				ctx:       context.TODO(),
				targetID:  "123",
				stressors: []string{"--cpu", "4"},
				image:     "test/stress-ng",
				pull:      true,
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				pullResponse := imagePullResponse{
					Status:   "ok",
					Error:    "no error",
					Progress: "done",
					ProgressDetail: struct {
						Current int `json:"current"`
						Total   int `json:"total"`
					}{
						Current: 100,
						Total:   100,
					},
				}
				pullResponseByte, _ := json.Marshal(pullResponse)
				readerResponse := bytes.NewReader(pullResponseByte)
				engine.EXPECT().ImagePull(ctx, image, imagetypes.PullOptions{}).Return(io.NopCloser(readerResponse), nil)
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{},
					},
				}
				conn.On("Close").Return(nil)
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil)
			},
			want:       "000",
			wantOutput: "stress completed",
		},
		{
			name: "stress test fail to pull image",
			args: args{
				ctx:   context.TODO(),
				image: "test/stress-ng",
				pull:  true,
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ImagePull(ctx, image, imagetypes.PullOptions{}).Return(io.NopCloser(bytes.NewReader([]byte("{}"))), errors.New("failed to pull image"))
			},
			wantErr: true,
		},
		{
			name: "stress test without pull image",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{},
					},
				}
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil)
				conn.On("Close").Return(nil)
			},
			want:       "000",
			wantOutput: "stress completed",
		},
		{
			name: "stress-ng exit with error",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{ExitCode: 1},
					},
				}
				conn.On("Close").Return(nil)
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil)
			},
			want:      "000",
			wantErrCh: true,
		},
		{
			name: "fail to inspect stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(nil)
				conn.On("Close").Return(nil)
				engine.EXPECT().ContainerInspect(ctx, "000").Return(ctypes.InspectResponse{}, errors.New("filed to inspect"))
			},
			want:       "000",
			wantOutput: "stress completed",
			wantErrCh:  true,
		},
		{
			name: "fail to start stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{
					Conn:   conn,
					Reader: bufio.NewReader(strings.NewReader("stress completed")),
				}, nil)
				engine.EXPECT().ContainerStart(ctx, "000", mock.Anything).Return(errors.New("failed to start"))
				conn.On("Close").Return(nil).Maybe()
				inspect := ctypes.InspectResponse{
					ContainerJSONBase: &ctypes.ContainerJSONBase{
						State: &ctypes.State{},
					},
				}
				engine.EXPECT().ContainerInspect(ctx, "000").Return(inspect, nil).Maybe()
			},
			want:    "000",
			wantErr: true,
		},
		{
			name: "fail to attach to stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{ID: "000"}, nil)
				engine.EXPECT().ContainerAttach(ctx, "000", mock.Anything).Return(types.HijackedResponse{}, errors.New("failed to attach"))
			},
			wantErr: true,
		},
		{
			name: "fail to create to stress-ng container",
			args: args{
				ctx: context.TODO(),
			},
			mockInit: func(ctx context.Context, engine *mocks.APIClient, conn *mockConn, targetID string, stressors []string, image string, pool bool) {
				engine.EXPECT().Info(mock.Anything).Return(system.Info{CgroupDriver: "cgroupfs"}, nil)
				engine.EXPECT().ContainerInspect(mock.Anything, targetID).Return(DetailsResponse(AsMap("ID", targetID)), nil).Once()
				engine.EXPECT().ContainerCreate(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return(ctypes.CreateResponse{}, errors.New("failed to create"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockEngine()
			mConn := &mockConn{}
			client := dockerClient{
				containerAPI: mockClient,
				imageAPI:     mockClient,
				systemAPI:    mockClient,
			}
			tt.mockInit(tt.args.ctx, mockClient, mConn, tt.args.targetID, tt.args.stressors, tt.args.image, tt.args.pull)
			got, got1, got2, err := client.stressContainerCommand(tt.args.ctx, tt.args.targetID, tt.args.stressors, tt.args.image, tt.args.pull, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("dockerClient.stressContainerCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("dockerClient.stressContainerCommand() got = %v, want %v", got, tt.want)
			}
			if err == nil && (got1 != nil || got2 != nil) {
				select {
				case output := <-got1:
					if output != tt.wantOutput {
						t.Errorf("dockerClient.stressContainerCommand() got = %v, from output channel, want %v", output, tt.wantOutput)
					}
				case err = <-got2:
					if (err != nil) != tt.wantErrCh {
						t.Errorf("dockerClient.stressContainerCommand() error in error channel = %v, wantErrCh %v", err, tt.wantErrCh)
					}
				}
			}
			mockClient.AssertExpectations(t)
			mConn.AssertExpectations(t)
		})
	}
}

func TestHTTPClient(t *testing.T) {
	tests := []struct {
		name      string
		daemonURL string
		tlsConfig *tls.Config
		wantErr   bool
	}{
		{
			name:      "tcp url with no TLS",
			daemonURL: "tcp://localhost:2375",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "http url with no TLS",
			daemonURL: "http://localhost:2375",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "tcp url with TLS",
			daemonURL: "tcp://localhost:2376",
			tlsConfig: &tls.Config{},
			wantErr:   false,
		},
		{
			name:      "https url with TLS",
			daemonURL: "https://localhost:2376",
			tlsConfig: &tls.Config{},
			wantErr:   false,
		},
		{
			name:      "unix socket",
			daemonURL: "unix:///var/run/docker.sock",
			tlsConfig: nil,
			wantErr:   false,
		},
		{
			name:      "invalid URL",
			daemonURL: "://invalid-url",
			tlsConfig: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := HTTPClient(tt.daemonURL, tt.tlsConfig)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				transport, ok := client.Transport.(*http.Transport)
				assert.True(t, ok)

				if tt.tlsConfig != nil {
					assert.Equal(t, tt.tlsConfig, transport.TLSClientConfig)
				}

				if tt.daemonURL != "" && strings.HasPrefix(tt.daemonURL, "unix:") {
					assert.NotNil(t, transport.DialContext)
				}
			}
		})
	}
}

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name    string
		address *url.URL
		tlsConf *tls.Config
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "http scheme",
			address: &url.URL{Scheme: "http", Host: "localhost:2375"},
			tlsConf: nil,
			timeout: 10 * time.Second,
			wantErr: false,
		},
		{
			name:    "https scheme with TLS",
			address: &url.URL{Scheme: "https", Host: "localhost:2376"},
			tlsConf: &tls.Config{InsecureSkipVerify: true},
			timeout: 10 * time.Second,
			wantErr: false,
		},
		{
			name:    "unix scheme",
			address: &url.URL{Scheme: "unix", Path: "/var/run/docker.sock"},
			tlsConf: nil,
			timeout: 10 * time.Second,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newHTTPClient(tt.address, tt.tlsConf, tt.timeout)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				transport, ok := client.Transport.(*http.Transport)
				assert.True(t, ok)

				if tt.tlsConf != nil {
					assert.Equal(t, tt.tlsConf, transport.TLSClientConfig)
				}

				assert.NotNil(t, transport.DialContext)

				if tt.address.Scheme == "unix" {
					assert.Equal(t, "http", tt.address.Scheme)
					assert.Equal(t, "unix.sock", tt.address.Host)
					assert.Equal(t, "", tt.address.Path)
				}
			}
		})
	}
}
