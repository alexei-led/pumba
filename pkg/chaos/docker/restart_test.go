package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "restart matching labeled containers by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				labels:  []string{"key=value"},
				timeout: 1 * time.Second,
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateLabeledTestContainers(3, map[string]string{"key": "value"}),
		},
		{
			name: "restart matching containers by filter with limit",
			fields: fields{
				pattern: "^c?",
				timeout: 1 * time.Second,
				limit:   2,
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "restart random matching container by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
			},
			args:     args{ctx: context.TODO(), random: true},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "no matching containers by names",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
			},
			args: args{ctx: context.TODO()},
		},
		{
			name: "error listing containers",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
			},
			args:    args{ctx: context.TODO()},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error restarting container",
			fields: fields{
				names:   []string{"c1", "c2", "c3"},
				timeout: 1 * time.Second,
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{restartError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
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
			if tt.errs.listError {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), opts).Return(nil, errors.New("ERROR"))
			} else {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), opts).Return(tt.expected, nil)
				if tt.expected != nil {
					restartCall := mockClient.EXPECT().RestartContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.timeout, tt.fields.dryRun)
					if tt.errs.restartError {
						restartCall.Return(errors.New("ERROR")).Once()
					} else if tt.args.random {
						restartCall.Return(nil).Once()
					} else {
						count := len(tt.expected)
						if tt.fields.limit > 0 && tt.fields.limit < count {
							count = tt.fields.limit
						}
						restartCall.Return(nil).Times(count)
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
