# Pumba: Chaos testing tool for Docker

[![Join the chat at https://gitter.im/pumba-chaos/Lobby](https://badges.gitter.im/pumba-chaos/Lobby.svg)](https://gitter.im/pumba-chaos/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

#### Build

[![Build Status](https://travis-ci.org/alexei-led/pumba.svg?branch=master)](https://travis-ci.org/alexei-led/pumba) [![Codefresh build status]( https://g.codefresh.io/api/badges/build?repoOwner=alexei-led&repoName=pumba&branch=master&pipelineName=pumba&accountName=codefresh-inc&type=cf-1)]( https://g.codefresh.io/repositories/alexei-led/pumba/builds?filter=trigger:build;branch:master;service:5a9d1dac81caf90001f95f9d~pumba) [![Go Report Card](https://goreportcard.com/badge/github.com/gaia-adm/pumba)](https://goreportcard.com/report/github.com/gaia-adm/pumba) [![codecov](https://codecov.io/gh/gaia-adm/pumba/branch/master/graph/badge.svg)](https://codecov.io/gh/gaia-adm/pumba) [![GitHub release](https://img.shields.io/github/release/gaia-adm/pumba.svg?no-cache)](https://github.com/gaia-adm/pumba/releases/tag/0.2.5)

#### Image

[![](https://badge.imagelayers.io/gaiaadm/pumba:master.svg)](https://imagelayers.io/?images=gaiaadm/pumba:master)  [![](https://images.microbadger.com/badges/image/gaiaadm/pumba.svg)](http://microbadger.com/images/gaiaadm/pumba) [![](https://images.microbadger.com/badges/version/gaiaadm/pumba.svg)](http://microbadger.com/images/gaiaadm/pumba) [![](https://images.microbadger.com/badges/commit/gaiaadm/pumba.svg)](http://microbadger.com/images/gaiaadm/pumba) [![Docker badge](https://img.shields.io/docker/pulls/gaiaadm/pumba.svg)](https://hub.docker.com/r/gaiaadm/pumba/) [![Anchore Image Overview](https://anchore.io/service/badges/image/77101bee4abccf2db02413002f25930b73bc6f5fea187b1b5ab1f0b538c1ba7a)](https://anchore.io/image/dockerhub/77101bee4abccf2db02413002f25930b73bc6f5fea187b1b5ab1f0b538c1ba7a?repo=gaiaadm%2Fpumba&tag=latest#overview)

## Demo

*TODO* : need to record a new video here ...

## Usage

You can download Pumba binary for your OS from [release](https://github.com/gaia-adm/pumba/releases) page.

```text
$ pumba help

Pumba version [VERSION](./blob/master/VERSION)
NAME:
   Pumba - Pumba is a resilience testing tool, that helps applications tolerate random Docker container failures: process, network and performance.

USAGE:
   pumba [global options] command [command options] containers (name, list of names, RE2 regex)

VERSION:
   [VERSION](./blob/master/VERSION) - `git rev-parse HEAD --short` and `build time`

COMMANDS:
     kill     kill specified containers
     netem    emulate the properties of wide area networks
     pause    pause all processes
     stop     stop containers
     rm       remove containers
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --host value, -H value      daemon socket to connect to (default: "unix:///var/run/docker.sock") [$DOCKER_HOST]
   --tls                       use TLS; implied by --tlsverify
   --tlsverify                 use TLS and verify the remote [$DOCKER_TLS_VERIFY]
   --tlscacert value           trust certs signed only by this CA (default: "/etc/ssl/docker/ca.pem")
   --tlscert value             client certificate for TLS authentication (default: "/etc/ssl/docker/cert.pem")
   --tlskey value              client key for TLS authentication (default: "/etc/ssl/docker/key.pem")
   --debug                     enable debug mode with verbose logging
   --json                      produce log in JSON format: Logstash and Splunk friendly
   --slackhook value           web hook url; send Pumba log events to Slack
   --slackchannel value        Slack channel (default #pumba) (default: "#pumba")
   --interval value, -i value  recurrent interval for chaos command; use with optional unit suffix: 'ms/s/m/h'
   --random, -r                randomly select single matching container from list of target containers
   --dry                       dry runl does not create chaos, only logs planned chaos commands
   --help, -h                  show help
   --version, -v               print the version
```

### Kill Container command

```text
$ pumba kill -h

NAME:
   pumba kill - kill specified containers

USAGE:
   pumba kill [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   send termination signal to the main process inside target container(s)

OPTIONS:
   --signal value, -s value  termination signal, that will be sent by Pumba to the main process inside target container(s) (default: "SIGKILL")
```

### Pause Container command

```text
$ pumba pause -h

NAME:
   pumba pause - pause all processes

USAGE:
   pumba pause [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   pause all running processes within target containers

OPTIONS:
   --duration value, -d value  pause duration: should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'
```

### Stop Container command

```text
$ pumba stop -h
NAME:
   pumba stop - stop containers

USAGE:
   pumba stop [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   stop the main process inside target containers, sending  SIGTERM, and then SIGKILL after a grace period

OPTIONS:
   --time value, -t value  seconds to wait for stop before killing container (default 10) (default: 10)
```

### Remove (rm) Container command

```text
$ pumba rm -h

NAME:
   pumba rm - remove containers

USAGE:
   pumba rm [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   remove target containers, with links and voluems

OPTIONS:
   --force, -f    force the removal of a running container (with SIGKILL, default: true)
   --links, -l    remove container links (default: false)
   --volumes, -v  remove volumes associated with the container (default: true)
```

### Network Emulation (netem) command

```text
$ pumba netem -h

NAME:
   Pumba netem - delay, loss, duplicate and re-order (run 'netem') packets, to emulate different network problems

USAGE:
   Pumba netem command [command options] [arguments...]

COMMANDS:
     delay      delay egress traffic
     loss
     duplicate
     corrupt
     rate       limit egress traffic

OPTIONS:
   --duration value, -d value   network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h'
   --interface value, -i value  network interface to apply delay on (default: "eth0")
   --target value, -t value     target IP filter; comma separated. netem will impact only on traffic to target IP(s)
   --tc-image value             Docker image with tc (iproute2 package); try 'gaiadocker/iproute2'
   --help, -h                   show help
```

#### Network Emulation Delay sub-command

```text
$ pumba netem delay -h

NAME:
   Pumba netem delay - delay egress traffic

USAGE:
   Pumba netem delay [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   delay egress traffic for specified containers; networks show variability so it is possible to add random variation; delay variation isn't purely random, so to emulate that there is a correlation

OPTIONS:
   --time value, -t value          delay time; in milliseconds (default: 100)
   --jitter value, -j value        random delay variation (jitter); in milliseconds; example: 100ms ± 10ms (default: 10)
   --correlation value, -c value   delay correlation; in percentage (default: 20)
   --distribution value, -d value  delay distribution, can be one of {<empty> | uniform | normal | pareto |  paretonormal}
```

#### Network Emulation Loss sub-commands

```text
$ pumba netem loss -h

NAME:
   Pumba netem loss - adds packet losses

USAGE:
   Pumba netem loss [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   adds packet losses, based on independent (Bernoulli) probability model
   see:  http://www.voiptroubleshooter.com/indepth/burstloss.html

OPTIONS:
   --percent value, -p value      packet loss percentage (default: 0)
   --correlation value, -c value  loss correlation; in percentage (default: 0)
```

```text
$ pumba netem loss-state -h

NAME:
   Pumba netem loss-state - adds packet losses, based on 4-state Markov probability model

USAGE:
   Pumba netem loss-state [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   adds a packet losses, based on 4-state Markov probability model
     state (1) – packet received successfully
     state (2) – packet received within a burst
     state (3) – packet lost within a burst
     state (4) – isolated packet lost within a gap
   see: http://www.voiptroubleshooter.com/indepth/burstloss.html

OPTIONS:
   --p13 value  probability to go from state (1) to state (3) (default: 0)
   --p31 value  probability to go from state (3) to state (1) (default: 100)
   --p32 value  probability to go from state (3) to state (2) (default: 0)
   --p23 value  probability to go from state (2) to state (3) (default: 100)
   --p14 value  probability to go from state (1) to state (4) (default: 0)
```

```text
$ pumba netem loss-gemodel -h

NAME:
   Pumba netem loss-gemodel - adds packet losses, according to the Gilbert-Elliot loss model

USAGE:
   Pumba netem loss-gemodel [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   adds packet losses, according to the Gilbert-Elliot loss model
   see: http://www.voiptroubleshooter.com/indepth/burstloss.html

OPTIONS:
   --pg value, -p value  transition probability into the bad state (default: 0)
   --pb value, -r value  transition probability into the good state (default: 100)
   --one-h value         loss probability in the bad state (default: 100)
   --one-k value         loss probability in the good state (default: 0)
```

```text
$ pumba netem rate -h

NAME:
   Pumba netem rate - rate limit egress traffic

USAGE:
   Pumba netem rate [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   rate limit egress traffic for specified containers

OPTIONS:
   --rate value, -r value            delay outgoing packets; in common units (default: "100kbit")
   --packetoverhead value, -p value  per packet overhead; in bytes (default: 0)
   --cellsize value, -s value        cell size of the simulated link layer scheme (default: 0)
   --celloverhead value, -c value    per cell overhead; in bytes (default: 0)
```

##### Examples

```text
# add 3 seconds delay for all outgoing packets on device `eth0` (default) of `mydb` Docker container for 5 minutes

$ pumba netem --duration 5m delay --time 3000 mydb
```

```text
# add a delay of 3000ms ± 30ms, with the next random element depending 20% on the last one,
# for all outgoing packets on device `eth1` of all Docker container, with name start with `hp`
# for 5 minutes

$ pumba netem --duration 5m --interface eth1 delay \
      --time 3000 \
      --jitter 30 \
      --correlation 20 \
    re2:^hp
```

```text
# add a delay of 3000ms ± 40ms, where variation in delay is described by `normal` distribution,
# for all outgoing packets on device `eth0` of randomly chosen Docker container from the list
# for 5 minutes

$ pumba --random netem --duration 5m \
    delay \
      --time 3000 \
      --jitter 40 \
      --distribution normal \
    container1 container2 container3
```

##### `tc` tool

Pumba uses `tc` Linux tool for network emulation. You have two options:

1. Make sure that container, you want to disturb, has `tc` tool available and properly installed (install `iproute2` package)
2. Use `--tc-image` option, with any `netem` command, to specify external Docker image with `tc` tool available. Pumba will create a new container from this image, adding `NET_ADMIN` capability to it and reusing target container network stack. You can try to use [gaiadocker/iproute2](https://hub.docker.com/r/gaiadocker/iproute2/) image (it's just Alpine Linux 3.3 with `iproute2` package installed)

**Note:** For Alpine Linux based image, you need to install `iproute2` package and also to create a symlink pointing to distribution files `ln -s /usr/lib/tc /lib/tc`.

### Running inside Docker container

If you choose to use Pumba Docker [image](https://hub.docker.com/r/gaiaadm/pumba/) on Linux, use the following command:

```text
# once in a 10 seconds, try to kill (with `SIGTERM` signal) all containers named **hp(something)**
# on same Docker host, where Pumba container is running

$ docker run -d -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba pumba --interval 10s kill --signal SIGTERM ^hp
```

**Note:** For Windows and OS X you will need to use `--host` argument, since there is no unix socket `/var/run/docker.sock` to mount.

### Running Pumba on Kubernetes cluster

If you are running Kubernetes >= 1.1.0. You can take advantage of DaemonSets to automatically deploy the Pumba on all your nodes.
On 1.1.x you'll need to explicitly enable the DaemonSets extension, see [documentation](http://kubernetes.io/v1.1/docs/admin/daemons.html#caveats).

You'll then be able to deploy the DaemonSet with the command

```sh
kubectl create -f pumba_kube.yml
```

If you are not running Kubernetes >= 1.1.0 or do not want to use DaemonSets, you can also run the Pumba as a regular docker container on each node you want to make chaos (see above)

## Build instructions

You can build Pumba with or without Go installed on your machine.

### Build using local Go environment

In order to build Pumba, you need to have Go 1.6+ setup on your machine.

Here is the approximate list of commands you will need to run:

```sh
# create required folder
cd $GOPATH
mkdir github.com/gaia-adm && cd github.com/gaia-adm

# clone pumba
git clone git@github.com:gaia-adm/pumba.git
cd pumba

# build pumba binary
./script/go_build.sh

# run tests and create HTML coverage report
./script/coverage.sh --html

# create pumba binaries for multiple platforms
./script/gox_build.sh
```

### Build using Docker

You do not have to install and configure Go in order to build and test Pumba project. Pumba uses Docker multistage build to create final tiny Docker image.

First of all clone Pumba git repository:

```sh
git clone git@github.com:gaia-adm/pumba.git
cd pumba
```

Now create a new Pumba Docker image.

```sh
docker build -t pumba -f Dockerfile .
```

## Used Libraries and Code

- Official Docker API for Go [docker/docker](https://github.com/docker/docker)
- Logging  [Sirupsen/logrus](https://github.com/Sirupsen/logrus)
- Command line app lib [codegangsta/cli](https://github.com/codegangsta/cli)

I've also borrowed some code from very good [CenturyLinkLabs/watchtower](https://github.com/CenturyLinkLabs/watchtower) project.

## License

Code is under the [Apache License v2](https://www.apache.org/licenses/LICENSE-2.0.txt).
