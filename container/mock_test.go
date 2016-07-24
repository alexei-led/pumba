package container

import (
	"reflect"
	"testing"
)

func TestMockInterface(t *testing.T) {
	iface := reflect.TypeOf((*Client)(nil)).Elem()
	mock := &MockClient{}

	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the Client interface")
	}
}
