package mockclient

import (
	"reflect"
	"testing"

	"github.com/gaia-adm/pumba/container"
)

func TestMockInterface(t *testing.T) {
	iface := reflect.TypeOf((*container.Client)(nil)).Elem()
	mock := &MockClient{}

	if !reflect.TypeOf(mock).Implements(iface) {
		t.Fatalf("Mock does not implement the Client interface")
	}
}
