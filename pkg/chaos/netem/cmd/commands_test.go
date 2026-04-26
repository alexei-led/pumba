package cmd

import (
	"context"
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
)

func fakeRuntime(t *testing.T) (chaos.Runtime, *container.MockClient) {
	t.Helper()
	want := container.NewMockClient(t)
	return chaos.Runtime(func() container.Client { return want }), want
}

func TestNewDelayCLICommand_StoresRuntime(t *testing.T) {
	rt, want := fakeRuntime(t)
	cli := NewDelayCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "delay", cli.Name)
	cmdCtx := &delayContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewLossCLICommand_StoresRuntime(t *testing.T) {
	rt, want := fakeRuntime(t)
	cli := NewLossCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "loss", cli.Name)
	cmdCtx := &lossContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewLossStateCLICommand_StoresRuntime(t *testing.T) {
	rt, want := fakeRuntime(t)
	cli := NewLossStateCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "loss-state", cli.Name)
	cmdCtx := &lossStateContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewLossGECLICommand_StoresRuntime(t *testing.T) {
	rt, want := fakeRuntime(t)
	cli := NewLossGECLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "loss-gemodel", cli.Name)
	cmdCtx := &lossGEContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewRateCLICommand_StoresRuntime(t *testing.T) {
	rt, want := fakeRuntime(t)
	cli := NewRateCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "rate", cli.Name)
	cmdCtx := &rateContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewDuplicateCLICommand_StoresRuntime(t *testing.T) {
	rt, want := fakeRuntime(t)
	cli := NewDuplicateCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "duplicate", cli.Name)
	cmdCtx := &duplicateContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}

func TestNewCorruptCLICommand_StoresRuntime(t *testing.T) {
	rt, want := fakeRuntime(t)
	cli := NewCorruptCLICommand(context.Background(), rt)
	assert.NotNil(t, cli)
	assert.Equal(t, "corrupt", cli.Name)
	cmdCtx := &corruptContext{context: context.Background(), runtime: rt}
	assert.Same(t, want, cmdCtx.runtime())
}
