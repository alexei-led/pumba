package container

import (
	"reflect"
	"testing"
)

func TestMockClient(t *testing.T) {
	mock := new(MockClient)
	iface := reflect.TypeOf((*Client)(nil)).Elem()

	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the Client interface")
	}
}
