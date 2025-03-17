package docker

import (
	"context"
	"errors"
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
		params  *chaos.GlobalParams
		command string
		args    []string
		limit   int
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
				params: &chaos.GlobalParams{
					Names: []string{"c1", "c2"},
				},
				command: "kill",
				args:    []string{"-9"},
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "exec matching labeled containers by names",
			fields: fields{
				params: &chaos.GlobalParams{
					Names:  []string{"c1", "c2", "c3"},
					Labels: []string{"key=value"},
				},
				command: "ls",
				args:    []string{"-la"},
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateLabeledTestContainers(3, map[string]string{"key": "value"}),
		},
		{
			name: "exec matching containers by filter with limit",
			fields: fields{
				params: &chaos.GlobalParams{
					Pattern: "^c?",
				},
				command: "kill",
				args:    []string{"-STOP", "1"},
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
				params: &chaos.GlobalParams{
					Names: []string{"c1", "c2", "c3"},
				},
				command: "kill",
				args:    []string{"1"},
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
				params: &chaos.GlobalParams{
					Names: []string{"c1", "c2", "c3"},
				},
				command: "kill",
				args:    []string{"1"},
			},
			args: args{
				ctx: context.TODO(),
			},
		},
		{
			name: "error listing containers",
			fields: fields{
				params: &chaos.GlobalParams{
					Names: []string{"c1", "c2", "c3"},
				},
				command: "kill",
				args:    []string{"1"},
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
				params: &chaos.GlobalParams{
					Names: []string{"c1", "c2", "c3"},
				},
				command: "kill",
				args:    []string{"1"},
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
			k := &execCommand{
				client:  mockClient,
				names:   tt.fields.params.Names,
				pattern: tt.fields.params.Pattern,
				labels:  tt.fields.params.Labels,
				command: tt.fields.command,
				args:    tt.fields.args,
				limit:   tt.fields.limit,
				dryRun:  tt.fields.params.DryRun,
			}
			opts := container.ListOpts{Labels: tt.fields.params.Labels}
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
				mockClient.On("ExecContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.command, tt.fields.args, tt.fields.params.DryRun).Return(nil)
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("ExecContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.command, tt.fields.args, tt.fields.params.DryRun)
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
