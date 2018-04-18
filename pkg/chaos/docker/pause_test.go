package docker

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexei-led/pumba/mocks"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

func TestNewPauseCommand(t *testing.T) {
	type args struct {
		client   container.Client
		names    []string
		pattern  string
		interval string
		duration string
		limit    int
		dryRun   bool
	}
	tests := []struct {
		name    string
		args    args
		want    ChaosCommand
		wantErr bool
	}{
		{
			name: "new pause command",
			args: args{
				names:    []string{"c1", "c2"},
				pattern:  "pattern",
				interval: "20s",
				duration: "10s",
				limit:    15,
			},
			want: &PauseCommand{
				names:    []string{"c1", "c2"},
				pattern:  "pattern",
				duration: 10 * time.Second,
				limit:    15,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPauseCommand(tt.args.client, tt.args.names, tt.args.pattern, tt.args.interval, tt.args.duration, tt.args.limit, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPauseCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPauseCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
		expected []container.Container
		wantErr  bool
		errs     wantErrors
	}{
		{
			name: "pause matching containers by names",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: testContainer3,
		},
		{
			name: "pause matching containers by filter with limit",
			fields: fields{
				pattern: "^c?",
				limit:   2,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: testContainer3,
		},
		{
			name: "pause random matching container by names",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{
				ctx:    context.TODO(),
				random: true,
			},
			expected: testContainer3,
		},
		{
			name: "no matching containers by names",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{
				ctx: context.TODO(),
			},
		},
		{
			name: "error listing containers",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{
				ctx: context.TODO(),
			},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error pausing container",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: testContainer3,
			wantErr:  true,
			errs:     wantErrors{pauseError: true},
		},
		{
			name: "error unpausing paused container",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: testContainer3,
			wantErr:  true,
			errs:     wantErrors{unpauseError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.Client)
			s := &PauseCommand{
				client:  mockClient,
				names:   tt.fields.names,
				pattern: tt.fields.pattern,
				limit:   tt.fields.limit,
				dryRun:  tt.fields.dryRun,
			}
			call := mockClient.On("ListContainers", tt.args.ctx, mock.AnythingOfType("container.Filter"))
			if tt.errs.listError {
				call.Return(tt.expected, errors.New("ERROR"))
				goto Invoke
			} else {
				call.Return(tt.expected, nil)
				if tt.expected == nil {
					goto Invoke
				}
			}
			if tt.args.random {
				mockClient.On("PauseContainer", tt.args.ctx, mock.AnythingOfType("container.Container"), tt.fields.dryRun).Return(nil)
				mockClient.On("UnpauseContainer", tt.args.ctx, mock.AnythingOfType("container.Container"), tt.fields.dryRun).Return(nil)
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("PauseContainer", tt.args.ctx, mock.AnythingOfType("container.Container"), tt.fields.dryRun)
						if tt.errs.pauseError {
							call.Return(errors.New("ERROR"))
							goto Invoke
						} else {
							call.Return(nil)
						}
						call = mockClient.On("UnpauseContainer", tt.args.ctx, mock.AnythingOfType("container.Container"), tt.fields.dryRun)
						if tt.errs.unpauseError {
							call.Return(errors.New("ERROR"))
							goto Invoke
						} else {
							call.Return(nil)
						}
					}
				}
			}
		Invoke:
			if err := s.Run(tt.args.ctx, tt.args.random); (err != nil) != tt.wantErr {
				t.Errorf("StopCommand.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}
