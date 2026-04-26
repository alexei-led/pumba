package cmd

import (
	"context"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
)

func TestNewLossCLICommand_StoresRuntime(t *testing.T) {
	want := container.NewMockClient(t)
	rt := chaos.Runtime(func() container.Client { return want })

	cli := NewLossCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "loss", cli.Name)

	cmdCtx := &lossContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewLossCLICommand_AcceptsNilRuntime(t *testing.T) {
	rt := chaos.Runtime(func() container.Client { return nil })
	assert.NotNil(t, NewLossCLICommand(context.Background(), rt))
}
