package cmd

import (
	"context"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
)

func TestNewStressCLICommand_StoresRuntime(t *testing.T) {
	want := container.NewMockClient(t)
	rt := chaos.Runtime(func() container.Client { return want })

	cli := NewStressCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "stress", cli.Name)

	cmdCtx := &stressContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewStressCLICommand_AcceptsNilRuntime(t *testing.T) {
	rt := chaos.Runtime(func() container.Client { return nil })
	assert.NotNil(t, NewStressCLICommand(context.Background(), rt))
}
