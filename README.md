# Pumba: Chaos testing tool for Docker

[![Circle CI](https://circleci.com/gh/gaia-adm/pumba.svg?style=svg)](https://circleci.com/gh/gaia-adm/pumba) [![Go Report Card](https://goreportcard.com/badge/github.com/gaia-adm/pumba)](https://goreportcard.com/report/github.com/gaia-adm/pumba) [![Coverage Status](https://coveralls.io/repos/github/gaia-adm/pumba/badge.svg?branch=master)](https://coveralls.io/github/gaia-adm/pumba?branch=master) [![](https://badge.imagelayers.io/gaiaadm/pumba:master.svg)](https://imagelayers.io/?images=gaiaadm/pumba:master) [![GitHub release](https://img.shields.io/github/release/gaia-adm/pumba.svg?no-cache)](https://github.com/gaia-adm/pumba/releases/tag/0.1.10)
[![Docker badge](https://img.shields.io/docker/pulls/gaiaadm/pumba.svg)](https://hub.docker.com/r/gaiaadm/pumba/)

## Demo

[![asciicast](https://asciinema.org/a/63l5ahg7fwkcq5y92gpq8b4tt.png)](https://asciinema.org/a/63l5ahg7fwkcq5y92gpq8b4tt?autoplay=1&speed=2)

## Usage

You can download Pumba binary for your OS from [release](https://github.com/gaia-adm/pumba/releases) page.

```
$ pumba help
```

### Pumba Chaos Commands

#### STOP command

`STOP` command will stop specified running container/s.

#### KILL command

`KILL` command will kill specified running container, sending `SIGKILL` Linux termination signal by default. It's possible to use other Linux termination signal with `KILL` command.
Pass `KILL:{SIGNALNAME}` (without braces) to Pumba though `chaos` option, and Pumba will send passed signal to main process running within your Docker container.

#### RM command

`RM` command will stop and force remove specified running container/s.

#### DISRUPT command - using Pumba to disrupt containers network

`DISRUPT` command will introduce network disruptions to a container's outgoing traffic (egress) using `netem` driver, and optionally limiting to traffic outgoing to specific target ip.

For testing your containers under specific network conditions, Pumba is using the linux [Network Emulation driver](http://www.linuxfoundation.org/collaborate/workgroups/networking/netem). As described in the above link, these commands affect the outgoing network traffic (egress), so for example, one may disrupt the backend API service or data service and then test the frontend to see the implications. In order to disrupt communication between specific containers, use the DISRUPT option to specify target container IP - Pumba will then filter the outgoing traffic and apply the netem effect only to the traffic going to the specific IP.

A few implementation points are important to understand:
* Netem commands are executed on containers using 'docker exec'. In order to change network driver settings, the container must have NET_ADMIN capability - this can be granted via the ['docker run' command](https://docs.docker.com/engine/reference/run/#/runtime-privilege-and-linux-capabilities), via [docker-compose.yml](https://docs.docker.com/compose/compose-file/#/cap-add-cap-drop) or otherwise.
* Pumba doesn't (currently) store 'chaos state' and therefore is not aware if the container has been already disrupted. As a result, subsequent calls will disrupt it again using the syntax 'tc qdisc add' instead of 'tc qdisc change'. Another result is that even when Pumba is stopped, the disrupted container is still disrupted. To manually fix the container for `eth0` network interface, use:
```
   docker exec container_name tc qdisc del dev eth0 root netem
```
* Generally, repeating the same disruption to the network of the same container has no effect. In a way, it will be best to use DISRUPT in random mode, combined with KILL so that disrupted containers will be removed from time to time.
* Using target IP means Pumba will add a priority queue and a filter on network interface, sending outgoing traffic with the specified IP destination to a secondary queue. The netem command is then applied to the secondary queue.

Example:
```
   pumba run --chaos mycontainer|10m|DISRUPT:eth2:delay 100ms:172.0.0.4

```
Once in 10 minutes, Pumba will create a delay in `eth2` network interface of `mycontainer` for egress traffic going to `172.0.0.4`.

### Running inside Docker container

If you choose to use Pumba Docker [image](https://hub.docker.com/r/gaiaadm/pumba/) on Linux, use the following command:

```
docker run -d -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba pumba run --chaos "re2:^hp|10s|KILL:SIGTERM"
```
The above command, once in a 10 seconds, tries to kill (with `SIGTERM` signal) all containers named **hp(something)** on same Docker host, where Pumba container is running.

**Note:** For Windows and OS X you will need to use `--host` argument, since there is no unix socket `/var/run/docker.sock` to mount.

### Running Pumba on Kubernetes cluster

If you are running Kubernetes >= 1.1.0. You can take advantage of DaemonSets to automatically deploy the Pumba on all your nodes.
On 1.1.x you'll need to explicitly enable the DaemonSets extension, see http://kubernetes.io/v1.1/docs/admin/daemons.html#caveats.

You'll then be able to deploy the DaemonSet with the command
```
$ kubectl create -f pumba_kube.yml
```

If you are not running Kubernetes >= 1.1.0 or do not want to use DaemonSets, you can also run the Pumba as a regular docker container on each node you want to make chaos (see above)

### Running Pumba on CoreOS cluster

If you are running CoreOS cluster. You can use `fleetctl` command to deploy Pumba service file on every CoreOS cluster node.
You'll then be able to deploy the Pumba service with the command
```
$ fleetctl start pumba_coreos.service
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

## Used Libraries and Code

- Docker Client [samalba/dockerclient](https://github.com/samalba/dockerclient)
- Logging  [Sirupsen/logrus](https://github.com/Sirupsen/logrus)
- Command line app lib [codegangsta/cli](https://github.com/codegangsta/cli)

I've also borrowed a code from very nice [CenturyLinkLabs/watchtower](https://github.com/CenturyLinkLabs/watchtower) project.

## License

Code is under the [Apache License v2](https://www.apache.org/licenses/LICENSE-2.0.txt).
