package mocks

import (
	"reflect"
	"testing"

	"github.com/alexei-led/pumba/pkg/container"
)

func TestMockClient(t *testing.T) {
	mock := new(Client)
	iface := reflect.TypeOf((*container.Client)(nil)).Elem()

	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the Client interface")
	}
}
