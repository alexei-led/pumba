package docker

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

func TestNewStopCommand(t *testing.T) {
	type args struct {
		client   container.Client
		names    []string
		pattern  string
		labels   []string
		restart  bool
		interval string
		duration string
		waitTime int
		limit    int
		dryRun   bool
	}
	tests := []struct {
		name    string
		args    args
		want    chaos.Command
		wantErr bool
	}{
		{
			name: "new stop command",
			args: args{
				names:    []string{"c1", "c2"},
				pattern:  "pattern",
				restart:  true,
				interval: "20s",
				duration: "10s",
				waitTime: 100,
				limit:    15,
			},
			want: &StopCommand{
				names:    []string{"c1", "c2"},
				pattern:  "pattern",
				restart:  true,
				duration: 10 * time.Second,
				waitTime: 100,
				limit:    15,
			},
		},
		{
			name: "new stop command with default wait",
			args: args{
				names:    []string{"c1", "c2"},
				pattern:  "pattern",
				duration: "10s",
				waitTime: 0,
				limit:    15,
			},
			want: &StopCommand{
				names:    []string{"c1", "c2"},
				pattern:  "pattern",
				duration: 10 * time.Second,
				waitTime: DeafultWaitTime,
				limit:    15,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewStopCommand(tt.args.client, tt.args.names, tt.args.pattern, tt.args.labels, tt.args.restart, tt.args.interval, tt.args.duration, tt.args.waitTime, tt.args.limit, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStopCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStopCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStopCommand_Run(t *testing.T) {
	type wantErrors struct {
		listError  bool
		stopError  bool
		startError bool
	}
	type fields struct {
		names    []string
		pattern  string
		restart  bool
		waitTime int
		limit    int
		dryRun   bool
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
			name: "stop matching containers by names",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop matching containers by names and restart",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
				restart:  true,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop matching containers by filter with limit",
			fields: fields{
				pattern:  "^c?",
				waitTime: 20,
				limit:    2,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop random matching container by names",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
			},
			args: args{
				ctx:    context.TODO(),
				random: true,
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop random matching container by names and restart",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
				restart:  true,
			},
			args: args{
				ctx:    context.TODO(),
				random: true,
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "no matching containers by names",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
			},
			args: args{
				ctx: context.TODO(),
			},
		},
		{
			name: "error listing containers",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 0,
			},
			args: args{
				ctx: context.TODO(),
			},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error stopping container",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{stopError: true},
		},
		{
			name: "error starting stopped container",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
				restart:  true,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{startError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(container.MockClient)
			s := &StopCommand{
				client:   mockClient,
				names:    tt.fields.names,
				pattern:  tt.fields.pattern,
				restart:  tt.fields.restart,
				waitTime: tt.fields.waitTime,
				limit:    tt.fields.limit,
				dryRun:   tt.fields.dryRun,
			}
			call := mockClient.On("ListContainers", tt.args.ctx, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts"))
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
				mockClient.On("StopContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.waitTime, tt.fields.dryRun).Return(nil)
				if tt.fields.restart {
					mockClient.On("StartContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(nil)
				}
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("StopContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.waitTime, tt.fields.dryRun)
						if tt.errs.stopError {
							call.Return(errors.New("ERROR"))
							goto Invoke
						} else {
							call.Return(nil)
						}
						if tt.fields.restart {
							call = mockClient.On("StartContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.dryRun)
							if tt.errs.startError {
								call.Return(errors.New("ERROR"))
								goto Invoke
							} else {
								call.Return(nil)
							}
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
