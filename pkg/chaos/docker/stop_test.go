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
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop matching containers by names and restart",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
				restart:  true,
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop matching containers by filter with limit",
			fields: fields{
				pattern:  "^c?",
				waitTime: 20,
				limit:    2,
			},
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop random matching container by names",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
			},
			args:     args{ctx: context.TODO(), random: true},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "stop random matching container by names and restart",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
				restart:  true,
			},
			args:     args{ctx: context.TODO(), random: true},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "no matching containers by names",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
			},
			args: args{ctx: context.TODO()},
		},
		{
			name: "error listing containers",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 0,
			},
			args:    args{ctx: context.TODO()},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error stopping container",
			fields: fields{
				names:    []string{"c1", "c2", "c3"},
				waitTime: 20,
			},
			args:     args{ctx: context.TODO()},
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
			args:     args{ctx: context.TODO()},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{startError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := container.NewMockClient(t)
			s := &stopCommand{
				client:   mockClient,
				names:    tt.fields.names,
				pattern:  tt.fields.pattern,
				restart:  tt.fields.restart,
				waitTime: tt.fields.waitTime,
				limit:    tt.fields.limit,
				dryRun:   tt.fields.dryRun,
			}
			if tt.errs.listError {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(nil, errors.New("ERROR"))
			} else {
				mockClient.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(tt.expected, nil)
				if tt.expected != nil {
					if tt.args.random {
						mockClient.EXPECT().StopContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.waitTime, tt.fields.dryRun).Return(nil).Once()
						if tt.fields.restart {
							mockClient.EXPECT().StartContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(nil).Once()
						}
					} else {
						count := len(tt.expected)
						if tt.fields.limit > 0 && tt.fields.limit < count {
							count = tt.fields.limit
						}
						if tt.errs.stopError {
							mockClient.EXPECT().StopContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.waitTime, tt.fields.dryRun).Return(errors.New("ERROR")).Once()
						} else {
							mockClient.EXPECT().StopContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.waitTime, tt.fields.dryRun).Return(nil).Times(count)
							if tt.fields.restart {
								if tt.errs.startError {
									mockClient.EXPECT().StartContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(errors.New("ERROR")).Times(count)
								} else {
									mockClient.EXPECT().StartContainer(mock.Anything, mock.AnythingOfType("*container.Container"), tt.fields.dryRun).Return(nil).Times(count)
								}
							}
						}
					}
				}
			}
			err := s.Run(tt.args.ctx, tt.args.random)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
