package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
			args:     args{ctx: context.TODO()},
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
			args:     args{ctx: context.TODO()},
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
			name: "error removing container",
			fields: fields{
				names: []string{"c1", "c2", "c3"},
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{removeError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			opts := container.RemoveOpts{
				Force:   tt.fields.force,
				Links:   tt.fields.links,
				Volumes: tt.fields.volumes,
				DryRun:  tt.fields.dryRun,
			}
			k := &removeCommand{
				client:  mockClient,
				names:   tt.fields.names,
				pattern: tt.fields.pattern,
				opts:    opts,
				limit:   tt.fields.limit,
			}
			if tt.errs.listError {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts container.ListOpts) bool { return opts.All == true })).Return(nil, errors.New("ERROR"))
			} else {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts container.ListOpts) bool { return opts.All == true })).Return(tt.expected, nil)
				if tt.expected != nil {
					removeCall := mockClient.EXPECT().RemoveContainer(mock.Anything, mock.AnythingOfType("*container.Container"), opts)
					if tt.errs.removeError {
						removeCall.Return(errors.New("ERROR")).Once()
					} else if tt.args.random {
						removeCall.Return(nil).Once()
					} else {
						count := len(tt.expected)
						if tt.fields.limit > 0 && tt.fields.limit < count {
							count = tt.fields.limit
						}
						removeCall.Return(nil).Times(count)
					}
				}
			}
			err := k.Run(tt.args.ctx, tt.args.random)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewRemoveCommand(t *testing.T) {
	tests := []struct {
		name    string
		params  *chaos.GlobalParams
		force   bool
		links   bool
		volumes bool
		limit   int
		want    *removeCommand
	}{
		{
			name:    "all opts true",
			params:  &chaos.GlobalParams{Names: []string{"c1"}, Pattern: "^c", Labels: []string{"k=v"}, DryRun: true},
			force:   true,
			links:   true,
			volumes: true,
			limit:   5,
			want: &removeCommand{
				names:   []string{"c1"},
				pattern: "^c",
				labels:  []string{"k=v"},
				opts:    container.RemoveOpts{Force: true, Links: true, Volumes: true, DryRun: true},
				limit:   5,
			},
		},
		{
			name:   "all opts false",
			params: &chaos.GlobalParams{Names: []string{"c1"}},
			want: &removeCommand{
				names: []string{"c1"},
				opts:  container.RemoveOpts{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			got := NewRemoveCommand(mockClient, tt.params, tt.force, tt.links, tt.volumes, tt.limit)
			cmd, ok := got.(*removeCommand)
			require.True(t, ok)
			tt.want.client = mockClient
			assert.True(t, reflect.DeepEqual(tt.want, cmd))
		})
	}
}
