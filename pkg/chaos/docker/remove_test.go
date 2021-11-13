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
			k := &RemoveCommand{
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
				t.Errorf("RemoveCommand.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestNewRemoveCommand(t *testing.T) {
	type args struct {
		client  container.Client
		names   []string
		pattern string
		labels  []string
		force   bool
		links   bool
		volumes bool
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
			name: "create new remove command",
			args: args{
				names:   []string{"c1", "c2"},
				force:   true,
				links:   true,
				volumes: false,
				limit:   10,
			},
			want: &RemoveCommand{
				names:   []string{"c1", "c2"},
				force:   true,
				links:   true,
				volumes: false,
				limit:   10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRemoveCommand(tt.args.client, tt.args.names, tt.args.pattern, tt.args.labels, tt.args.force, tt.args.links, tt.args.volumes, tt.args.limit, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRemoveCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewRemoveCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
