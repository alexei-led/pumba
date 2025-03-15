package iptables

import (
	"testing"

	"github.com/alexei-led/pumba/pkg/chaos"
)

func TestNTHParamValidation(t *testing.T) {
	gp := &chaos.GlobalParams{}
	p := &Params{}

	_, err := NewLossCommand(nil, gp, p, "nth", 0.0, 0, 0)
	if err == nil {
		t.Error("expected an error because input of every is incorrect")
	}
	_, err = NewLossCommand(nil, gp, p, "nth", 0.0, 1, 0)
	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}
	_, err = NewLossCommand(nil, gp, p, "nth", 0.0, 1, -1)
	if err == nil {
		t.Error("expected an error because input of packet is incorrect")
	}
	_, err = NewLossCommand(nil, gp, p, "nth", 0.0, 1, 1)
	if err == nil {
		t.Error("expected an error because input of packet is incorrect")
	}
	_, err = NewLossCommand(nil, gp, p, "nth", 0.0, 2, 1)
	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}
}

func TestRandomParamValidation(t *testing.T) {
	gp := &chaos.GlobalParams{}
	p := &Params{}

	_, err := NewLossCommand(nil, gp, p, "random", -1.0, 0, 0)
	if err == nil {
		t.Error("expected an error because input of probability is incorrect")
	}
	_, err = NewLossCommand(nil, gp, p, "random", 1.01, 0, 0)
	if err == nil {
		t.Error("expected an error because input of probability is incorrect")
	}
	_, err = NewLossCommand(nil, gp, p, "random", 0.0, 1, 0)
	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}
	_, err = NewLossCommand(nil, gp, p, "random", 1.0, 1, 0)
	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}
}
