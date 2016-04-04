# Pumba: Chaos testing tool for Docker

[![Circle CI](https://circleci.com/gh/gaia-adm/pumba.svg?style=svg)](https://circleci.com/gh/gaia-adm/pumba) [![Go Report Card](https://goreportcard.com/badge/github.com/gaia-adm/pumba)](https://goreportcard.com/report/github.com/gaia-adm/pumba) [![Coverage Status](https://coveralls.io/repos/github/gaia-adm/pumba/badge.svg?branch=master)](https://coveralls.io/github/gaia-adm/pumba?branch=master)

## Usage

```
NAME:
   pumba - Pumba is a resiliency tool that helps applications tolerate random Docker container failures.

USAGE:
   pumba [global options] command [command options] [arguments...]

VERSION:
   0.1.1

COMMANDS:
GLOBAL OPTIONS:
   --host, -H "unix:///var/run/docker.sock"  daemon socket to connect to [$DOCKER_HOST]
   --tls                                     use TLS; implied by --tlsverify
   --tlsverify                               TLS and verify the remote [$DOCKER_TLS_VERIFY]
   --tlscacert "/etc/ssl/docker/ca.pem"      trust certs signed only by this CA
   --tlscert "/etc/ssl/docker/cert.pem"      client certificate for TLS authentication
   --tlskey "/etc/ssl/docker/key.pem"        client key for TLS authentication
   --debug                                   enable debug mode with verbose logging
   --chaos_cmd [--chaos_cmd option]	         chaos command: `container(s,)/re2:regex|interval(s/m/h postfix)|STOP/KILL(:SIGNAL)/RM`
   --help, -h                                show help
   --version, -v                             print the version
```

## Build instructions

You can build Pumba with or without Go installed on your machine.

### Build using local Go environment

In order to build Pumba, you need to have Go 1.6+ setup on your machine.

Here is the approximate list of commands you will need to run:

```
cd $GOPATH
mkdir github.com/gaia-adm && cd github.com/gaia-adm
git clone git@github.com:gaia-adm/pumba.git
cd pumba
glide install
go build -v
```

### Build using Docker builder image

You do not have to install and configure Go in order to build and test Pumba project. Pubma builder Docker image contains Go 1.6 and all tools required to build and test Pumba.

First of all clone Pumba git repository:
```
git clone git@github.com:gaia-adm/pumba.git
cd pumba
```

Now create a Pumba builder Docker image.
```
docker build -t pumba/builder -f Build.Dockerfile .
```

Now you can use `pumba/builder` to build, test (with coverage) and deploy Pumba.

To build a new Pumba binary run the following command:
```
docker run --rm -v "$PWD":/go/src/github.com/gaia-adm/pumba -w /go/src/github.com/gaia-adm/pumba pumba/builder script/go_build.sh
```

To build new Pumba binaries for multiple platforms run the following command (using `gox` tool):
```
docker run --rm -v "$PWD":/go/src/github.com/gaia-adm/pumba -w /go/src/github.com/gaia-adm/pumba pumba/builder script/gox_build.sh
```

To run all Pumba tests and generate coverage report run the following command:
```
docker run --rm -v "$PWD":/go/src/github.com/gaia-adm/pumba -w /go/src/github.com/gaia-adm/pumba pumba/builder script/coverage.sh --html
```
