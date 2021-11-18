package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/mock"
)

func TestRemoveCommand_Run(t *testing.T) {
	type wantErrors struct {
		listError   bool
		removeError bool
	}
	type fields struct {
		names   []string
		pattern string
		force   bool
		links   bool
		volumes bool
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
			name: "remove matching containers by names",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
				force: true,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "remove matching containers by filter with limit",
			fields: fields{
				pattern: "^c?",
				force:   true,
				links:   true,
				limit:   2,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "remove random matching container by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				force:   true,
				links:   true,
				volumes: true,
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
			name: "error removing container",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{removeError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(container.MockClient)
			k := &removeCommand{
				client:  mockClient,
				names:   tt.fields.names,
				pattern: tt.fields.pattern,
				force:   tt.fields.force,
				links:   tt.fields.links,
				volumes: tt.fields.volumes,
				limit:   tt.fields.limit,
				dryRun:  tt.fields.dryRun,
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
				mockClient.On("RemoveContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.force, tt.fields.links, tt.fields.volumes, tt.fields.dryRun).Return(nil)
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("RemoveContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.force, tt.fields.links, tt.fields.volumes, tt.fields.dryRun)
						if tt.errs.removeError {
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
				t.Errorf("removeCommand.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}
