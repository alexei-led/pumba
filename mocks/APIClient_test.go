package mocks

import (
	"reflect"
	"testing"

	"github.com/docker/docker/client"
)

func TestMockAPIClient(t *testing.T) {
	mock := new(APIClient)

	iface := reflect.TypeOf((*client.ContainerAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the ContainerAPIClient interface")
	}
	iface = reflect.TypeOf((*client.ImageAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the ImageAPIClient interface")
	}
	iface = reflect.TypeOf((*client.NetworkAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the NetworkAPIClient interface")
	}
	iface = reflect.TypeOf((*client.VolumeAPIClient)(nil)).Elem()
	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the VolumeAPIClient interface")
	}
}
