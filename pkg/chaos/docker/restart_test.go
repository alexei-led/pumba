package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

func TestRestartCommand_Run(t *testing.T) {
	type wantErrors struct {
		listError    bool
		restartError bool
	}
	type fields struct {
		names   []string
		pattern string
		labels  []string
		timeout time.Duration
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
			name: "restart matching containers by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "restart matching labeled containers by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				labels:  []string{"key=value"},
				timeout: 1 * time.Second,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateLabeledTestContainers(3, map[string]string{"key": "value"}),
		},
		{
			name: "restart matching containers by filter with limit",
			fields: fields{
				pattern: "^c?",
				timeout: 1 * time.Second,
				limit:   2,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "restart random matching container by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
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
				timeout: 1 * time.Second,
			},
			args: args{
				ctx: context.TODO(),
			},
		},
		{
			name: "error listing containers",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
			},
			args: args{
				ctx: context.TODO(),
			},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error restarting container",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{restartError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(container.MockClient)
			k := &restartCommand{
				client:  mockClient,
				names:   tt.fields.names,
				pattern: tt.fields.pattern,
				labels:  tt.fields.labels,
				timeout: 1 * time.Second,
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
				mockClient.On("RestartContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.timeout, tt.fields.dryRun).Return(nil)
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("RestartContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.timeout, tt.fields.dryRun)
						if tt.errs.restartError {
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
				t.Errorf("restartCommand.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}
