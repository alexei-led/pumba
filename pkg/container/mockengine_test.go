package container

import (
	"reflect"
	"testing"

	"github.com/alexei-led/pumba/pkg/container/mocks"
	egineapi "github.com/docker/docker/client"
)

func TestMockEngineInterface(t *testing.T) {
	mock := new(mocks.APIClient)

	iface := reflect.TypeOf((*egineapi.ContainerAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the ContainerAPIClient interface")
	}
	iface = reflect.TypeOf((*egineapi.ImageAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the ImageAPIClient interface")
	}
	iface = reflect.TypeOf((*egineapi.NetworkAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the NetworkAPIClient interface")
	}
	iface = reflect.TypeOf((*egineapi.VolumeAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the VolumeAPIClient interface")
	}
}
