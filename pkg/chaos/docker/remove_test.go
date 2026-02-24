package docker

import (
	"context"
	"errors"
	"testing"

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
			if tt.errs.listError {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts container.ListOpts) bool { return opts.All == true })).Return(nil, errors.New("ERROR"))
			} else {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts container.ListOpts) bool { return opts.All == true })).Return(tt.expected, nil)
				if tt.expected != nil {
					removeCall := mockClient.EXPECT().RemoveContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.force, tt.fields.links, tt.fields.volumes, tt.fields.dryRun)
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
