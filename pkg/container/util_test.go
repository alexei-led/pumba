package container

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListNContainers(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*MockClient) []*Container
		call      func(context.Context, *MockClient) ([]*Container, error)
		wantLen   int
		wantErr   bool
	}{
		{
			name: "all_flag_passed_when_true",
			setupMock: func(m *MockClient) []*Container {
				expected := CreateTestContainers(2)
				m.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts ListOpts) bool {
					return opts.All == true
				})).Return(expected, nil)
				return expected
			},
			call: func(ctx context.Context, m *MockClient) ([]*Container, error) {
				return ListNContainersAll(ctx, m, []string{"c0", "c1"}, "", nil, 0, true)
			},
			wantLen: 2,
		},
		{
			name: "all_flag_false_by_default",
			setupMock: func(m *MockClient) []*Container {
				expected := CreateTestContainers(2)
				m.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts ListOpts) bool {
					return opts.All == false
				})).Return(expected, nil)
				return expected
			},
			call: func(ctx context.Context, m *MockClient) ([]*Container, error) {
				return ListNContainers(ctx, m, []string{"c0", "c1"}, "", nil, 0)
			},
			wantLen: 2,
		},
		{
			name: "error_propagates",
			setupMock: func(m *MockClient) []*Container {
				m.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).
					Return(([]*Container)(nil), errors.New("docker error"))
				return nil
			},
			call: func(ctx context.Context, m *MockClient) ([]*Container, error) {
				return ListNContainersAll(ctx, m, []string{"c0"}, "", nil, 0, true)
			},
			wantErr: true,
		},
		{
			name: "limit_truncates_results",
			setupMock: func(m *MockClient) []*Container {
				expected := CreateTestContainers(5)
				m.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).
					Return(expected, nil)
				return expected
			},
			call: func(ctx context.Context, m *MockClient) ([]*Container, error) {
				return ListNContainersAll(ctx, m, []string{"c0", "c1", "c2", "c3", "c4"}, "", nil, 2, true)
			},
			wantLen: 2,
		},
		{
			name: "limit_equal_to_count",
			setupMock: func(m *MockClient) []*Container {
				expected := CreateTestContainers(3)
				m.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).
					Return(expected, nil)
				return expected
			},
			call: func(ctx context.Context, m *MockClient) ([]*Container, error) {
				return ListNContainersAll(ctx, m, []string{"c0", "c1", "c2"}, "", nil, 3, false)
			},
			wantLen: 3,
		},
		{
			name: "labels_filter_applied",
			setupMock: func(m *MockClient) []*Container {
				expected := CreateTestContainers(1)
				m.EXPECT().ListContainers(mock.Anything, mock.AnythingOfType("container.FilterFunc"),
					mock.MatchedBy(func(opts ListOpts) bool {
						return opts.All == false && len(opts.Labels) == 2 && opts.Labels[0] == "env=prod" && opts.Labels[1] == "tier=web"
					})).Return(expected, nil)
				return expected
			},
			call: func(ctx context.Context, m *MockClient) ([]*Container, error) {
				return ListNContainers(ctx, m, []string{"c0"}, "", []string{"env=prod", "tier=web"}, 0)
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockClient)
			tt.setupMock(mockClient)

			containers, err := tt.call(context.TODO(), mockClient)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, containers)
			} else {
				require.NoError(t, err)
				assert.Len(t, containers, tt.wantLen)
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestRandomContainer(t *testing.T) {
	tests := []struct {
		name       string
		containers []*Container
		expectNil  bool
	}{
		{
			name:       "nil slice returns nil",
			containers: nil,
			expectNil:  true,
		},
		{
			name:       "empty slice returns nil",
			containers: []*Container{},
			expectNil:  true,
		},
		{
			name: "single container returns that container",
			containers: []*Container{
				{ContainerID: "only-one", Labels: map[string]string{}, Networks: map[string]NetworkLink{}},
			},
		},
		{
			name:       "multiple containers returns one of them",
			containers: CreateTestContainers(5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RandomContainer(tt.containers)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Contains(t, tt.containers, result)
			}
		})
	}
}
