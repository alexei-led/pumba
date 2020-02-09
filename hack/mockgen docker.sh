#!/bin/sh

echo "Regenerating vendor"
go mod vendor -v

# (re)generate mock for Docker APIClient interface
echo "generating mocks for Docker APIs"
mockery -name APIClient -dir vendor/github.com/docker/docker/client -name ^*APIClient

# remove vendor folder
rm -rf vendor