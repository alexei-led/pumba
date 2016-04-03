# Pumba: Chaos testing tool for Docker

[![Circle CI](https://circleci.com/gh/gaia-adm/pumba.svg?style=svg)](https://circleci.com/gh/gaia-adm/pumba) [![Go Report Card](https://goreportcard.com/badge/github.com/gaia-adm/pumba)](https://goreportcard.com/report/github.com/gaia-adm/pumba) [![Coverage Status](https://coveralls.io/repos/github/gaia-adm/pumba/badge.svg?branch=master)](https://coveralls.io/github/gaia-adm/pumba?branch=master)

## Build instructions

First, build Pumba's development environment.

```
docker build -t pumba/builder -f Build.Dockerfile .
```

Now you can use `pumba/builder` to build, test (with coverage) and deploy Pumba.

To build a Pumba new binary:
```
docker run --rm -v "$PWD":/go/src/github.com/gaia-adm/pumba -w /go/src/github.com/gaia-adm/pumba pumba/builder go build -v
```
