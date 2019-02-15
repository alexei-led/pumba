#!/bin/sh

# build Docker with mockery
docker build -t local/mockery -f hack/mockgen.Dockerfile .
alias mockery="docker run -it --rm -v ${PWD}:/go/src/github.com/alexei-led/pumba local/mockery"

# generate vendor folder
echo "Regenerating vendor"
GO111MODULE=on go mod vendor -v

# (re)generate mock for Docker APIClient interface
echo "generating mocks"
mockery -name APIClient -dir vendor/github.com/docker/docker/client -name ^*APIClient

# (re)generate mock for Command
mockery -dir pkg/chaos/docker -all

# (re)generate mock for Client
mockery -dir pkg/container -inpkg -all

# remove vendor folder
rm -rf vendor