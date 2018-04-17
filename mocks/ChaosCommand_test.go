package mocks

import (
	"reflect"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos/docker"
)

func TestMockChaosCommand(t *testing.T) {
	mock := new(ChaosCommand)
	iface := reflect.TypeOf((*docker.ChaosCommand)(nil)).Elem()

	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the ChaosCommand interface")
	}
}
