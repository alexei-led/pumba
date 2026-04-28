package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPauseCommand_Run(t *testing.T) {
	type wantErrors struct {
		listError    bool
		pauseError   bool
		unpauseError bool
	}
	type fields struct {
		names   []string
		pattern string
		limit   int
		dryRun  bool
	}
	type args struct {
		ctx    context.Context
		random bool
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected []*container.Container
		wantErr  bool
		errs     wantErrors
	}{
		{
			name: "pause matching containers by names",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "pause matching containers by filter with limit",
			fields: fields{
				pattern: "^c?",
				limit:   2,
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "pause random matching container by names",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args:     args{ctx: context.TODO(), random: true},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "no matching containers by names",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{ctx: context.TODO()},
		},
		{
			name: "error listing containers",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args:    args{ctx: context.TODO()},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error pausing container",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{pauseError: true},
		},
		{
			name: "error unpausing paused container",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{unpauseError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			s := &pauseCommand{
				client:  mockClient,
				names:   tt.fields.names,
				pattern: tt.fields.pattern,
				limit:   tt.fields.limit,
				dryRun:  tt.fields.dryRun,
			}
			if tt.errs.listError {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(nil, errors.New("ERROR"))
			} else {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(tt.expected, nil)
				if tt.expected != nil {
					if tt.args.random {
						mockClient.EXPECT().PauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(nil).Once()
						mockClient.EXPECT().UnpauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(nil).Once()
					} else {
						count := len(tt.expected)
						if tt.fields.limit > 0 && tt.fields.limit < count {
							count = tt.fields.limit
						}
						if tt.errs.pauseError {
							mockClient.EXPECT().PauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(errors.New("ERROR")).Once()
						} else {
							mockClient.EXPECT().PauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(nil).Times(count)
							if tt.errs.unpauseError {
								mockClient.EXPECT().UnpauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(errors.New("ERROR")).Times(count)
							} else {
								mockClient.EXPECT().UnpauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(nil).Times(count)
							}
						}
					}
				}
			}
			err := s.Run(tt.args.ctx, tt.args.random)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewPauseCommand(t *testing.T) {
	tests := []struct {
		name     string
		params   *chaos.GlobalParams
		duration time.Duration
		limit    int
		want     *pauseCommand
	}{
		{
			name:     "fields propagated",
			params:   &chaos.GlobalParams{Names: []string{"c1"}, Pattern: "^c", Labels: []string{"k=v"}, DryRun: true},
			duration: 3 * time.Second,
			limit:    2,
			want: &pauseCommand{
				names:    []string{"c1"},
				pattern:  "^c",
				labels:   []string{"k=v"},
				duration: 3 * time.Second,
				limit:    2,
				dryRun:   true,
			},
		},
		{
			name:     "zero duration and no limit",
			params:   &chaos.GlobalParams{Names: []string{"c1"}},
			duration: 0,
			limit:    0,
			want: &pauseCommand{
				names: []string{"c1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			got := NewPauseCommand(mockClient, tt.params, tt.duration, tt.limit)
			cmd, ok := got.(*pauseCommand)
			require.True(t, ok)
			tt.want.client = mockClient
			assert.True(t, reflect.DeepEqual(tt.want, cmd))
		})
	}
}

func TestPauseCommand_Run_CtxCancelUnpausesContainers(t *testing.T) {
	mockClient := container.NewMockClient(t)
	containers := container.CreateTestContainers(2)

	ctx, cancel := context.WithCancel(context.Background())

	p := &pauseCommand{
		client:   mockClient,
		names:    []string{"c1", "c2"},
		duration: 10 * time.Second, // long — timer must not fire
	}

	mockClient.EXPECT().
		ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).
		Return(containers, nil)
	mockClient.EXPECT().
		PauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).
		Return(nil).Times(2)
	// UnpauseContainer must still be called even after ctx cancellation.
	mockClient.EXPECT().
		UnpauseContainer(mock.Anything, mock.AnythingOfType("*container.Container"), false).
		Return(nil).Times(2)

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := p.Run(ctx, false)
	assert.NoError(t, err)
}
