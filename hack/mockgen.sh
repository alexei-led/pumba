#!/bin/sh

# get mockery tool
go get -u github.com/vektra/mockery/.../

# (re)generate mock for Docker APIClient interface
mockery -name APIClient -dir vendor/github.com/docker/docker/client

# (re)generate mock for ChaosCommand
mockery -dir pkg/chaos/docker -all

# (re)generate mock for Client
mockery -dir pkg/container -all