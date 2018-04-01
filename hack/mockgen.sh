#!/bin/sh

# get mockery tool
go get -u github.com/vektra/mockery/.../

# (re)generate mock for Docker APIClient interface
mockery -name APIClient -dir vendor/github.com/docker/docker/client -output ./pkg/container/mocks