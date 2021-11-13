package docker

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

func TestExecCommand_Run(t *testing.T) {
	type wantErrors struct {
		listError bool
		execError bool
	}
	type fields struct {
		names   []string
		pattern string
		labels  []string
		command string
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
			name: "exec matching containers by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				command: "kill 1",
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "exec matching labeled containers by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				labels:  []string{"key=value"},
				command: "kill 1",
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateLabeledTestContainers(3, map[string]string{"key": "value"}),
		},
		{
			name: "exec matching containers by filter with limit",
			fields: fields{
				pattern: "^c?",
				command: "kill -STOP 1",
				limit:   2,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "exec random matching container by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				command: "kill 1",
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
				names:   []string{"c1", "c2", "c3"},
				command: "kill 1",
			},
			args: args{
				ctx: context.TODO(),
			},
		},
		{
			name: "error listing containers",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				command: "kill 1",
			},
			args: args{
				ctx: context.TODO(),
			},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error execing container",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				command: "kill 1",
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{execError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(container.MockClient)
			k := &ExecCommand{
				client:  mockClient,
				names:   tt.fields.names,
				pattern: tt.fields.pattern,
				labels:  tt.fields.labels,
				command: tt.fields.command,
				limit:   tt.fields.limit,
				dryRun:  tt.fields.dryRun,
			}
			opts := container.ListOpts{Labels: tt.fields.labels}
			call := mockClient.On("ListContainers", tt.args.ctx, mock.AnythingOfType("container.FilterFunc"), opts)
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
				mockClient.On("ExecContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.command, tt.fields.dryRun).Return(nil)
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("ExecContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.command, tt.fields.dryRun)
						if tt.errs.execError {
							call.Return(errors.New("ERROR"))
							goto Invoke
						} else {
							call.Return(nil)
						}
					}
				}
			}
		Invoke:
			if err := k.Run(tt.args.ctx, tt.args.random); (err != nil) != tt.wantErr {
				t.Errorf("ExecCommand.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestNewExecCommand(t *testing.T) {
	type args struct {
		client  container.Client
		names   []string
		pattern string
		labels  []string
		command string
		limit   int
		dryRun  bool
	}
	tests := []struct {
		name    string
		args    args
		want    chaos.Command
		wantErr bool
	}{
		{
			name: "create new exec command",
			args: args{
				names:   []string{"c1", "c2"},
				command: "kill -TERM 1",
				limit:   10,
			},
			want: &ExecCommand{
				names:   []string{"c1", "c2"},
				command: "kill -TERM 1",
				limit:   10,
			},
		},
		{
			name: "empty command",
			args: args{
				names:   []string{"c1", "c2"},
				command: "",
			},
			want: &ExecCommand{
				names:   []string{"c1", "c2"},
				command: "kill 1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExecCommand(tt.args.client, tt.args.names, tt.args.pattern, tt.args.labels, tt.args.command, tt.args.limit, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExecCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewExecCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
