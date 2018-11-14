#!/bin/sh

# build Docker with mockery
docker build -t local/mockery -f hack/mockgen.Dockerfile .
alias mockery="docker run -it --rm -v ${PWD}:/go/src/github.com/alexei-led/pumba local/mockery"

# (re)generate mock for Docker APIClient interface
mockery -name APIClient -dir vendor/github.com/docker/docker/client

# (re)generate mock for Command
mockery -dir pkg/chaos/docker -all

# (re)generate mock for Client
mockery -dir pkg/container -inpkg -all