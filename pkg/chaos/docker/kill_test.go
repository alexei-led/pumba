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

func TestKillCommand_Run(t *testing.T) {
	type wantErrors struct {
		listError bool
		killError bool
	}
	type fields struct {
		names   []string
		pattern string
		labels  []string
		signal  string
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
			name: "kill matching containers by names",
			fields: fields{
				names:  []string{"c1", "c2", "c3"},
				signal: "SIGKILL",
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "kill matching labeled containers by names",
			fields: fields{
				names:  []string{"c1", "c2", "c3"},
				labels: []string{"key=value"},
				signal: "SIGKILL",
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateLabeledTestContainers(3, map[string]string{"key": "value"}),
		},
		{
			name: "kill matching containers by filter with limit",
			fields: fields{
				pattern: "^c?",
				signal:  "SIGSTOP",
				limit:   2,
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
		},
		{
			name: "kill random matching container by names",
			fields: fields{
				names:  []string{"c1", "c2", "c3"},
				signal: "SIGKILL",
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
				names:  []string{"c1", "c2", "c3"},
				signal: "SIGKILL",
			},
			args: args{
				ctx: context.TODO(),
			},
		},
		{
			name: "error listing containers",
			fields: fields{
				names:  []string{"c1", "c2", "c3"},
				signal: "SIGKILL",
			},
			args: args{
				ctx: context.TODO(),
			},
			wantErr: true,
			errs:    wantErrors{listError: true},
		},
		{
			name: "error killing container",
			fields: fields{
				names:  []string{"c1", "c2", "c3"},
				signal: "SIGKILL",
			},
			args: args{
				ctx: context.TODO(),
			},
			expected: container.CreateTestContainers(3),
			wantErr:  true,
			errs:     wantErrors{killError: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(container.MockClient)
			k := &killCommand{
				client:  mockClient,
				names:   tt.fields.names,
				pattern: tt.fields.pattern,
				labels:  tt.fields.labels,
				signal:  tt.fields.signal,
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
				mockClient.On("KillContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.signal, tt.fields.dryRun).Return(nil)
			} else {
				for i := range tt.expected {
					if tt.fields.limit == 0 || i < tt.fields.limit {
						call = mockClient.On("KillContainer", tt.args.ctx, mock.AnythingOfType("*container.Container"), tt.fields.signal, tt.fields.dryRun)
						if tt.errs.killError {
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
				t.Errorf("killCommand.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestNewKillCommand(t *testing.T) {
	type args struct {
		client container.Client
		params *chaos.GlobalParams
		signal string
		limit  int
	}
	tests := []struct {
		name    string
		args    args
		want    chaos.Command
		wantErr bool
	}{
		{
			name: "create new kill command",
			args: args{
				params: &chaos.GlobalParams{Names: []string{"c1", "c2"}},
				signal: "SIGTERM",
				limit:  10,
			},
			want: &killCommand{
				names:  []string{"c1", "c2"},
				signal: "SIGTERM",
				limit:  10,
			},
		},
		{
			name: "invalid signal",
			args: args{
				params: &chaos.GlobalParams{Names: []string{"c1", "c2"}},
				signal: "SIGNONE",
			},
			wantErr: true,
		},
		{
			name: "empty signal",
			args: args{
				params: &chaos.GlobalParams{Names: []string{"c1", "c2"}},
				signal: "",
			},
			want: &killCommand{
				names:  []string{"c1", "c2"},
				signal: DefaultKillSignal,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewKillCommand(tt.args.client, tt.args.params, tt.args.signal, tt.args.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKillCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewKillCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
