package container

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestApplyContainerFilter_SkipsPumba(t *testing.T) {
	labels := map[string]string{"com.gaiaadm.pumba": "true"}
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "pumba-container", "Labels", labels)),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}
	flt := filter{Names: []string{"pumba-container"}, Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.False(t, fn(c))
}

func TestApplyContainerFilter_SkipsPumbaSkip(t *testing.T) {
	labels := map[string]string{"com.gaiaadm.pumba.skip": "true"}
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "skip-container", "Labels", labels)),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}
	flt := filter{Names: []string{"skip-container"}, Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.False(t, fn(c))
}

func TestApplyContainerFilter_MatchesByName(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "target")),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}

	flt := filter{Names: []string{"target"}, Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(c))
}

func TestApplyContainerFilter_MatchesByNameWithAllFlag(t *testing.T) {
	target := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "target")),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}
	other := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "other")),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}

	flt := filter{Names: []string{"target"}, Opts: ListOpts{All: true}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(target))
	assert.False(t, fn(other))
}

func TestApplyContainerFilter_MatchesByPatternWithAllFlag(t *testing.T) {
	matching := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "app-web-1")),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}
	nonMatching := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "db-postgres-1")),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}

	flt := filter{Pattern: "^app-", Opts: ListOpts{All: true}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(matching))
	assert.False(t, fn(nonMatching))
}

func TestApplyContainerFilter_MatchesByPattern(t *testing.T) {
	c := &Container{
		ContainerInfo: DetailsResponse(AsMap("Name", "app-web")),
		ImageInfo:     ImageDetailsResponse(AsMap()),
	}

	flt := filter{Pattern: "^app-", Opts: ListOpts{All: false}}
	fn := applyContainerFilter(flt)

	assert.True(t, fn(c))
}

func TestListNContainersAll_PassesAllFlagToDocker(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(2)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts ListOpts) bool {
		return opts.All == true
	})).Return(expected, nil)

	containers, err := ListNContainersAll(context.TODO(), mockClient, []string{"c0", "c1"}, "", nil, 0, true)

	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	mockClient.AssertExpectations(t)
}

func TestListNContainersAll_DefaultDoesNotPassAllFlag(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(2)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.MatchedBy(func(opts ListOpts) bool {
		return opts.All == false
	})).Return(expected, nil)

	containers, err := ListNContainers(context.TODO(), mockClient, []string{"c0", "c1"}, "", nil, 0)

	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	mockClient.AssertExpectations(t)
}

func TestListNContainersAll_ReturnsError(t *testing.T) {
	mockClient := new(MockClient)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(([]*Container)(nil), errors.New("docker error"))

	containers, err := ListNContainersAll(context.TODO(), mockClient, []string{"c0"}, "", nil, 0, true)

	assert.Error(t, err)
	assert.Nil(t, containers)
	mockClient.AssertExpectations(t)
}

func TestListNContainersAll_RespectsLimit(t *testing.T) {
	mockClient := new(MockClient)
	expected := CreateTestContainers(5)

	mockClient.On("ListContainers", mock.Anything, mock.AnythingOfType("container.FilterFunc"), mock.AnythingOfType("container.ListOpts")).Return(expected, nil)

	containers, err := ListNContainersAll(context.TODO(), mockClient, []string{"c0", "c1", "c2", "c3", "c4"}, "", nil, 2, true)

	assert.NoError(t, err)
	assert.Len(t, containers, 2)
	mockClient.AssertExpectations(t)
}
