#!/bin/sh

# (re)generate mock for Command
mockery -dir pkg/chaos/docker -all

# (re)generate mock for Client
mockery -dir pkg/container -inpkg -all
