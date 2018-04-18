package mocks

import (
	"reflect"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
)

func TestMockChaosCommand(t *testing.T) {
	mock := new(Command)
	iface := reflect.TypeOf((*chaos.Command)(nil)).Elem()

	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the Command interface")
	}
}
