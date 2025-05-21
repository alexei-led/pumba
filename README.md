# Pumba: chaos testing tool for Docker and containerd [![Tweet](https://img.shields.io/twitter/url/http/shields.io.svg?style=social)](https://twitter.com/intent/tweet?text=Breaking%20Docker%20and%20containerd%20containers%20on%20purpose%20with%20Pumba&url=https://github.com/alexei-led/pumba&via=alexeiled&hashtags=docker,containerd,chaosengineering,chaos,breakthingsonpurpose,kubernetes)

Pumba is a chaos testing command line tool for Docker and containerd.
Pumba disturbs your containers by:

- Crashing containerized applications
- Emulating network failures (latency, packet loss, etc.)
- Manipulating both incoming and outgoing network traffic
- Stress-testing container resources (CPU, memory, I/O)
- Creating complex, realistic network chaos scenarios

![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/alexei-led/pumba)
[![](https://github.com/alexei-led/pumba/workflows/Pumba%20CI/badge.svg)](https://github.com/alexei-led/pumba/actions?query=workflow%3A"Release")
[![Go Report Card](https://goreportcard.com/badge/github.com/alexei-led/pumba)](https://goreportcard.com/report/github.com/alexei-led/pumba)
[![codecov](https://codecov.io/gh/alexei-led/pumba/branch/master/graph/badge.svg)](https://codecov.io/gh/alexei-led/pumba)
[![Docker Pulls](https://img.shields.io/docker/pulls/gaiaadm/pumba.svg)](https://hub.docker.com/r/gaiaadm/pumba/)
[![Docker Image Size](https://img.shields.io/docker/image-size/gaiaadm/pumba/latest.svg)](https://hub.docker.com/r/gaiaadm/pumba/)

![pumba](docs/img/pumba_logo.png "They CALL me... MISTER PIG!")

## Prerequisites

**Important:**:
- For Docker: Minimal required Docker version `v18.06.0`.
- For containerd: Pumba is tested with containerd `v1.3.x` and later. Ensure your `containerd` installation is operational and the Pumba binary can access the containerd socket.

## Demo

[![asciicast](https://asciinema.org/a/82428.png)](https://asciinema.org/a/82428)

## Usage

You can download Pumba binary for your OS from
[release](https://github.com/alexei-led/pumba/releases) page.

```text
$ pumba help

Pumba version [VERSION](./blob/master/VERSION)
NAME:
   Pumba - Pumba is a resilience testing tool, that helps applications tolerate random container failures (Docker or containerd): process, network and performance.

USAGE:
   pumba [global options] command [command options] containers (name, list of names, RE2 regex)

VERSION:
   [VERSION](./blob/master/VERSION) - `git rev-parse HEAD --short` and `build time`

COMMANDS:
   kill      kill specified containers
   exec      exec specified containers
   restart   restart specified containers
   stop      stop containers
   pause     pause all processes
   rm        remove containers
   stress    stress test a specified containers
   netem     emulate the properties of wide area networks
   iptables  apply IPv4 packet filter on incoming IP packets
   help, h   Shows a list of commands or help for one command
GLOBAL OPTIONS:
   --host value, -H value       daemon socket to connect to (default: "unix:///var/run/docker.sock", used for Docker runtime) [$DOCKER_HOST]
   --tls                        use TLS; implied by --tlsverify (Docker runtime only)
   --tlsverify                  use TLS and verify the remote (Docker runtime only) [$DOCKER_TLS_VERIFY]
   --tlscacert value            trust certs signed only by this CA (default: "/etc/ssl/docker/ca.pem")
   --tlscert value              client certificate for TLS authentication (default: "/etc/ssl/docker/cert.pem")
   --tlskey value               client key for TLS authentication (default: "/etc/ssl/docker/key.pem")
   --log-level value, -l value  set log level (debug, info, warning(*), error, fatal, panic) (default: "warning") [$LOG_LEVEL]
   --json, -j                   produce log in JSON format: Logstash and Splunk friendly [$LOG_JSON]
   --slackhook value            web hook url; send Pumba log events to Slack
   --slackchannel value         Slack channel (default #pumba) (default: "#pumba")
   --interval value, -i value   recurrent interval for chaos command; use with optional unit suffix: 'ms/s/m/h' (default: 0s)
   --label value                filter containers by labels, e.g '--label key=value' (multiple labels supported)
   --random, -r                 randomly select single matching container from list of target containers
   --dry-run                    dry run does not create chaos, only logs planned chaos commands [$DRY-RUN]
   --skip-error                 skip chaos command error and retry to execute the command on next interval tick
   --runtime value              Container runtime to use: 'docker' or 'containerd' (default: "docker")
   --containerd-address value   containerd address (socket path) (default: "/run/containerd/containerd.sock")
   --containerd-namespace value containerd namespace (default: "k8s.io")
   --help, -h                   show help
   --version, -v                print the version
```

### Runtime Configuration

Pumba supports both Docker and containerd runtimes. You can select the runtime using the `--runtime` global option.

*   **Docker (default)**: Pumba will attempt to connect to the Docker daemon.
    *   `--host`: Specifies the Docker daemon socket (default: `unix:///var/run/docker.sock`).
    *   TLS options (`--tls`, `--tlsverify`, etc.) are applicable for Docker TCP connections.

*   **containerd**: To use Pumba with containerd, specify `--runtime containerd`.
    *   `--containerd-address`: Path to the containerd socket (default: `/run/containerd/containerd.sock`).
    *   `--containerd-namespace`: Containerd namespace to operate within (default: `k8s.io`).

**Quick start with containerd (Linux):**
```bash
# Start a container using containerd's ctr tool
ctr -n demo run -d --name ping docker.io/library/alpine:latest ping 1.1.1.1

# Run Pumba against it
pumba --runtime containerd \
  --containerd-address /run/containerd/containerd.sock \
  --containerd-namespace demo \
  netem --duration 30s delay --time 300 ping
```

On macOS, containerd typically runs inside Docker Desktop. Expose the socket or
run Pumba inside the Docker Desktop VM and use the same command, adjusting the
`--containerd-address` to the VM's socket path.

See [examples/pumba_containerd_delay.sh](examples/pumba_containerd_delay.sh) for a
scripted demo.

**Note on `stress` command with containerd**: The `stress` command relies on cgroup access. When targeting containerd containers, Pumba attempts to place the `stress-ng` helper container into the target container's cgroup. This requires Pumba to have sufficient privileges to interact with containerd and for the `stress-ng` helper image to be compatible. The default `stress-image` (`alexeiled/stress-ng:latest-ubuntu`) should work if Pumba has appropriate host access or equivalent privileges.

### Kill Container command

```text
pumba kill -h

NAME:
   pumba kill - kill specified containers

USAGE:
   pumba [global options] kill [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   send termination signal to the main process inside target container(s)

OPTIONS:
   --signal value, -s value  termination signal, that will be sent by Pumba to the main process inside target container(s) (default: "SIGKILL")
   --limit value, -l value   limit to number of container to kill (0: kill all matching) (default: 0)
```

### Pause Container command

```text
pumba pause -h

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
pumba stop -h
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
pumba rm -h

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
pumba netem -h

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
   --tc-image value             Docker image with tc (iproute2 package); try 'ghcr.io/alexei-led/pumba-debian-nettools'
   --help, -h                   show help
```

#### Network Emulation Delay sub-command

```text
pumba netem delay -h

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
pumba netem loss -h

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
pumba netem loss-state -h

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
pumba netem loss-gemodel -h

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
pumba netem rate -h

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

#### Network Emulation Duplicate sub-commands

```text
pumba netem duplicate -h

NAME:
   Pumba netem duplicate - adds duplicate packets

USAGE:
   Pumba netem duplicate [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   adds duplicate packets, based on independent (Bernoulli) probability model
   see:  http://www.voiptroubleshooter.com/indepth/burstloss.html

OPTIONS:
   --percent value, -p value      packet duplicate percentage (default: 0)
   --correlation value, -c value  duplicate correlation; in percentage (default: 0)
```

#### Network Emulation Corrupt sub-commands

```text
pumba netem corrupt -h

NAME:
   Pumba netem corrupt - adds corrupt packets

USAGE:
   Pumba netem corrupt [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   adds corrupt packets, based on independent (Bernoulli) probability model
   see:  http://www.voiptroubleshooter.com/indepth/burstloss.html

OPTIONS:
   --percent value, -p value      packet corrupt percentage (default: 0)
   --correlation value, -c value  corrupt correlation; in percentage (default: 0)
```

##### Examples

```text
# add 3 seconds delay for all outgoing packets on device `eth0` (default) of `mydb` Docker container for 5 minutes

pumba netem --duration 5m delay --time 3000 mydb
```

```text
# add a delay of 3000ms ± 30ms, with the next random element depending 20% on the last one,
# for all outgoing packets on device `eth1` of all Docker container, with name start with `test`
# for 5 minutes

pumba netem --duration 5m --interface eth1 delay \
      --time 3000 \
      --jitter 30 \
      --correlation 20 \
    "re2:^test"
```

```text
# add a delay of 3000ms ± 40ms, where variation in delay is described by `normal` distribution,
# for all outgoing packets on device `eth0` of randomly chosen Docker container from the list
# for 5 minutes

pumba --random netem --duration 5m \
    delay \
      --time 3000 \
      --jitter 40 \
      --distribution normal \
    container1 container2 container3
```

```text
# Corrupt 10% of the packets from the `mydb` Docker container for 5 minutes

pumba netem --duration 5m corrupt --percent 10 mydb
```

```text
# Using the multi-arch nettools image explicitly
# This is useful when you need to ensure both netem and iptables commands use the same image

pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
    --duration 5m \
    delay --time 1000 \
    --jitter 100 \
    myapp
```

For more examples of combining netem with iptables commands, see the [Advanced Network Chaos Scenarios](#advanced-network-chaos-scenarios)
section.

##### Network Tools Images

Pumba uses the `tc` Linux tool for network emulation and `iptables` for packet filtering.
You have two options:

1. Make sure that the container you want to disturb has the required tools available and
   properly installed (install `iproute2` and `iptables` packages)
2. Use provided network tools images with the `--tc-image` option (for netem commands)
   or `--iptables-image` option (for iptables commands)

   Pumba will create a new container from this image, adding `NET_ADMIN`
   capability to it and reusing the target container's network stack.

#### Combined NetTools Images

By default, Pumba now uses multi-tool container images that include both `tc` and `iptables` tools:

- `ghcr.io/alexei-led/pumba-alpine-nettools:latest` - Alpine-based image with both tc and iptables
- `ghcr.io/alexei-led/pumba-debian-nettools:latest` - Debian-based image with both tc and iptables

These images provide several benefits:

- **Efficiency**: Both the `netem` and `iptables` commands can use the same container image
- **Multi-architecture**: Images are built for both `amd64` and `arm64` architectures
- **Command reuse**: A neutral entrypoint keeps the helper container alive between commands

**Usage Example**:

```bash
# Use the same nettools image for both netem and iptables commands
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest delay --time 100 mycontainer
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest loss --probability 0.2 mycontainer 
```

#### Architecture Support

The nettools images are built for multiple CPU architectures:

- `amd64` (x86_64) - Standard 64-bit Intel/AMD architecture
- `arm64` (aarch64) - 64-bit ARM architecture (Apple M1/M2, AWS Graviton, etc.)

Docker will automatically pull the correct image for your architecture.

#### Building Network Tools Images

You can build the network tools images locally using the provided Makefile commands:

```bash
# Build single-arch images for local testing
make build-local-nettools

# Build multi-architecture images locally (doesn't push)
make build-nettools-images

# Build and push the multi-architecture images to GitHub Container Registry
make push-nettools-images
```

Before pushing to GitHub Container Registry, you need to authenticate:

1. Create a GitHub Personal Access Token with `write:packages` permission
2. Set environment variables and login:

```bash
# Set your GitHub username and token
export GITHUB_USERNAME=your-github-username
export GITHUB_TOKEN=your-personal-access-token

# Login to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin

# Run the make command with the environment variables
make push-nettools-images
```

You can also set the variables inline with the make command:

```bash
GITHUB_USERNAME=your-github-username GITHUB_TOKEN=your-personal-access-token make push-nettools-images
```

### IPTables command

```text
pumba iptables -h
NAME:
   Pumba iptables - emulate loss of incoming packets, all ports and address arguments will result in seperate rules
USAGE:
   Pumba iptables command [command options] containers (name, list of names, or RE2 regex if prefixed with "re2:"
COMMANDS:
   loss  adds iptables rules to generate packet loss on ingress traffic
OPTIONS:
   --duration value, -d value             network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h' (default: 0s)
   --interface value, -i value            network interface to apply input rules on (default: "eth0")
   --protocol value, -p value             protocol to apply input rules on (any, udp, tcp or icmp) (default: "any")
   --source value, --src value, -s value  source IP filter; supports multiple IPs; supports CIDR notation
   --destination value, --dest value      destination IP filter; supports multiple IPs; supports CIDR notation
   --src-port value, --sport value        source port filter; supports multiple ports (comma-separated)
   --dst-port value, --dport value        destination port filter; supports multiple ports (comma-separated)
   --iptables-image value                 Docker image with iptables and tc tools (default: "ghcr.io/alexei-led/pumba-alpine-nettools:latest")
   --pull-image                           force pull iptables-image
   --help, -h                             show help
```

#### IPTables loss command

```text
pumba iptables loss -h
NAME:
   Pumba iptables loss - adds iptables rules to generate packet loss on ingress traffic
USAGE:
   Pumba iptables loss [command options] containers (name, list of names, or RE2 regex if prefixed with "re2:"
DESCRIPTION:
   adds packet losses on ingress traffic by setting iptable statistic rules
   see:  https://www.man7.org/linux/man-pages/man8/iptables-extensions.8.html
OPTIONS:
   --mode value         matching mode, supported modes are random and nth (default: "random")
   --probability value  set the probability for a packet to me matched in random mode, between 0.0 and 1.0 (default: 0)
   --every value        match one packet every nth packet, works only with nth mode (default: 0)
   --packet value       set the initial counter value (0 <= packet <= n-1, default 0) for nth mode (default: 0)
```

#### Using the `iptables` Commands

Pumba's `iptables` command allows you to simulate packet loss for incoming network traffic, with powerful filtering options. This can be
used to test application resilience to network issues.

##### Examples

```bash
# Drop 20% of incoming packets for a container named "web"
pumba iptables loss --probability 0.2 web
```

```bash
# Drop every 5th packet coming from IP 192.168.1.100 to container "api" on port 8080
pumba iptables loss --mode nth --every 5 --protocol tcp --source 192.168.1.100 --dst-port 8080 api
```

```bash
# Drop 15% of incoming ICMP packets (ping) for all containers with names matching "database"
pumba iptables loss --probability 0.15 --protocol icmp "re2:database"
```

```bash
# Complex example: Drop 25% of TCP traffic coming to port 443 from a specific subnet, for 30 seconds
pumba iptables --duration 30s --protocol tcp --source 10.0.0.0/24 --dst-port 443 \
    loss --probability 0.25 mycontainer
```

##### `iptables` Image Requirements

Pumba uses the nettools images (which include both `tc` and `iptables`) for filtering incoming network traffic.
You have two options:

1. Make sure the target container has the `iptables` tool installed
   (install the `iptables` package)

2. Use the `--iptables-image` option to specify a Docker image with
   the `iptables` tool.

   Pumba will create a helper container from this image with `NET_ADMIN`
   capability and reuse the target container's network stack.

   The recommended images are:
    - `ghcr.io/alexei-led/pumba-alpine-nettools:latest` (Alpine-based)
    - `ghcr.io/alexei-led/pumba-debian-nettools:latest` (Debian-based)

   Both images support multiple architectures (amd64, arm64).

### Advanced Network Chaos Scenarios

Pumba allows you to create complex and realistic network chaos scenarios by combining multiple network manipulation commands. This is
particularly useful for simulating real-world network conditions where multiple issues might occur simultaneously.

#### Asymmetric Network Conditions

In real networks, upload and download speeds/quality often differ. You can simulate this using a combination of `netem` for outgoing traffic
and `iptables` for incoming traffic:

```bash
# Add delay to outgoing traffic (slow uploads)
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 5m delay --time 500 myapp &

# Add packet loss to incoming traffic (unreliable downloads)
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 5m loss --probability 0.1 myapp &
```

#### Combined Network Degradation

Test how your application handles multiple concurrent network issues:

```bash
# Limit bandwidth and add packet corruption
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 10m rate --rate 1mbit myapp &

# Add packet loss to incoming traffic
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 10m loss --probability 0.05 myapp &
```

#### Testing Microservices Resilience

Use Pumba to test how your microservices architecture responds to network failures between specific services:

```bash
# Add high latency between service A and service B
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --target service-b-ip --duration 5m delay --time 2000 --jitter 500 service-a &

# Add packet loss from service B to service C
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --source service-c-ip --duration 5m loss --probability 0.2 service-b &
```

#### Example Script

You can find a complete example script for combined chaos testing in the [examples directory](examples/pumba_combined.sh).

For detailed guidance on advanced network chaos testing scenarios, best practices, and troubleshooting, see
the [Advanced Network Chaos Testing Documentation](docs/advanced-network-chaos.md).

### Stress testing containers

Pumba can inject [stress-ng](https://kernel.ubuntu.com/~cking/stress-ng/)
testing tool into a target container(s) and control the stress test run.
With Docker, this uses the `dockhack` script and cgroup manipulation.
With containerd, Pumba attempts to place the helper container into the target's cgroup.

```text
NAME:
   pumba stress - stress test a specified containers

USAGE:
   pumba stress [command options] containers (name, list of names, or RE2 regex if prefixed with "re2:")

DESCRIPTION:
   stress test target container(s)

OPTIONS:
   --duration value, -d value  stress duration: must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'
   --stress-image value        Docker image with stress-ng tool, cgroup-bin and docker packages, and dockhack script (default: "alexeiled/stress-ng:latest-ubuntu")
   --pull-image                pull stress-image from Docker registry
   --stressors value           stress-ng stressors; see https://kernel.ubuntu.com/~cking/stress-ng/ (default: "--cpu 4 --timeout 60s")
```

#### stress-ng image requirements

Pumba uses `alexeiled/stress-ng:latest-ubuntu` as the default `--stress-image`.
This image is Ubuntu-based with a statically linked `stress-ng` tool.

When using Docker runtime, the image must include the following for Pumba's `stress` command to work as designed with `dockhack`:

1. `stress-ng` tool (in `$PATH`)
1. Bash shell
1. [`dockhack`](https://github.com/tavisrudd/dockhack) helper Bash script (in
   `$PATH`)
1. `docker` client CLI tool (runnable without `sudo`)
1. `cgexec` tool, available from `cgroups-tools` or/and `cgroup-bin` packages

For containerd, the primary requirement for the helper image is to have `stress-ng` in its `$PATH`. The cgroup interaction is handled by Pumba placing the helper container in the target's cgroup.

### Running inside Docker container

If you choose to use Pumba Docker
[image](https://hub.docker.com/r/gaiaadm/pumba/) on Linux, use the following
command:

```text
# run 10 Docker containers named test_(index)
for i in `seq 1 10`; do docker run -d --name test_$i --rm alpine tail -f /dev/null; done

# once in a 10 seconds, try to kill (with `SIGKILL` signal) all containers named **test(something)**
# on same Docker host, where Pumba container is running (ensure --runtime docker or default)

$ docker run -it --rm  -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba --runtime docker --interval=10s --random --log-level=info kill --signal=SIGKILL "re2:^test"

```

**Note:** from version `0.6` Pumba Docker image is a `scratch` Docker image,
that contains only single `pumba` binary file and `ENTRYPOINT` set to the
`pumba` command. This image is primarily intended for Docker runtime chaos.

For targeting `containerd` containers, it's generally recommended to run the Pumba binary directly on the node or from a privileged pod that has access to the containerd socket (e.g., `/run/containerd/containerd.sock` or `/run/k3s/containerd/containerd.sock`).

**Note:** For Windows and OS X (using Docker Desktop), you will need to use `--host` argument when targeting Docker, since
there is no unix socket `/var/run/docker.sock` to mount directly for the Pumba Docker image. `containerd` support on these platforms via Pumba would depend on how `containerd` is exposed and accessible.

### Running Pumba on Kubernetes cluster

If you are running Kubernetes, you can take advantage of DaemonSets to
automatically deploy the Pumba on selected K8s nodes, using `nodeSelector` or
`nodeAffinity`, see
[Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/).
When running as a DaemonSet, ensure Pumba has access to the correct container runtime socket and any necessary privileges. For `containerd`, this typically means mounting the `containerd.sock` and running Pumba with appropriate permissions (e.g., as root or with specific capabilities if further restricted).

You'll then be able to deploy the DaemonSet with the command:

```sh
kubectl create -f deploy/pumba_kube.yml
```
(You may need to modify `pumba_kube.yml` to specify the runtime and its parameters, or to mount the correct socket for containerd.)

K8s automatically assigns labels to containers (whether managed by Docker or containerd via a CRI plugin), and you can use Pumba's
`--label` filter to create chaos for specific Pods and Namespaces.

K8s auto-assigned container labels, that can be used by Pumba:

```yaml
"io.kubernetes.container.name": "test-container"
"io.kubernetes.pod.name": "test-pod"
"io.kubernetes.pod.namespace": "test-namespace"
```

It's possible to run multiple Pumba commands in the same DaemonSet using
multiple Pumba containers, see `deploy/pumba_kube.yml` example.

If you are not running Kubernetes >= 1.1.0 or do not want to use DaemonSets, you
can also run the Pumba as a regular docker container on each node you want to
make chaos (see above)

**Note:** running `pumba netem` commands on minikube clusters will not work,
because the sch_netem kernel module is missing in the minikube VM!

## Build instructions

You can build Pumba with or without Go installed on your machine.

### Build using local Go environment

In order to build Pumba, you need to have Go 1.6+ setup on your machine.

Here is the approximate list of commands you will need to run:

```sh
# create required folder
cd $GOPATH
mkdir github.com/alexei-led && cd github.com/alexei-led

# clone pumba
git clone git@github.com:alexei-led/pumba.git
cd pumba

# build pumba binary
make

# run tests and create HTML coverage report
make test-coverage

# create pumba binaries for multiple platforms
make release
```

### Build using Docker

You do not have to install and configure Go in order to build and test Pumba
project.
Pumba uses Docker multistage build to create final tiny Docker image.

First of all clone Pumba git repository:

```sh
git clone git@github.com:alexei-led/pumba.git
cd pumba
```

Now create a new Pumba Docker image.

```sh
DOCKER_BUILDKIT=1 docker build -t pumba -f docker/Dockerfile .
```

### Exec Container command

```text
pumba exec -h

NAME:
   pumba exec - exec specified containers

USAGE:
   pumba [global options] exec [command options] containers (name, list of names, RE2 regex)

DESCRIPTION:
   send command to target container(s)

OPTIONS:
   --command value, -s value  shell command, that will be sent by Pumba to the target container(s) (default: "kill 1")
   --args value, -a value     additional arguments for the command (can be repeated for multiple arguments)
   --limit value, -l value    limit number of container to exec (0: exec all matching) (default: 0)
```

#### Examples

```bash
# Execute default command (kill 1) in container named web
pumba exec web
```

```bash
# Execute a custom command (echo) with a single argument in container named web
pumba exec --command "echo" --args "hello" web
```

```bash
# Execute ls with multiple arguments in all containers matching regex
# Use repeated --args flags for multiple arguments
pumba exec --command "ls" --args "-la" --args "/etc" "re2:^api.*"
```

```bash
# Limit execution to only 2 containers even if more match
pumba exec --command "touch" --args "/tmp/test-file" --limit 2 "re2:.*"
```

##### Network Tools Images

Pumba uses the `tc` Linux tool for network emulation and `iptables` for packet filtering.
You have two options:

1. Make sure that the container you want to disturb has the required tools available and
   properly installed (install `iproute2` and `iptables` packages)
2. Use provided network tools images with the `--tc-image` option (for netem commands)
   or `--iptables-image` option (for iptables commands)

   Pumba will create a new container from this image, adding `NET_ADMIN`
   capability to it and reusing the target container's network stack.

#### Combined NetTools Images

By default, Pumba now uses multi-tool container images that include both `tc` and `iptables` tools:

- `ghcr.io/alexei-led/pumba-alpine-nettools:latest` - Alpine-based image with both tc and iptables
- `ghcr.io/alexei-led/pumba-debian-nettools:latest` - Debian-based image with both tc and iptables

These images provide several benefits:

- **Efficiency**: Both the `netem` and `iptables` commands can use the same container image
- **Multi-architecture**: Images are built for both `amd64` and `arm64` architectures
- **Command reuse**: A neutral entrypoint keeps the helper container alive between commands

**Usage Example**:

```bash
# Use the same nettools image for both netem and iptables commands
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest delay --time 100 mycontainer
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest loss --probability 0.2 mycontainer 
```

#### Architecture Support

The nettools images are built for multiple CPU architectures:

- `amd64` (x86_64) - Standard 64-bit Intel/AMD architecture
- `arm64` (aarch64) - 64-bit ARM architecture (Apple M1/M2, AWS Graviton, etc.)

Docker will automatically pull the correct image for your architecture.

#### Building Network Tools Images

You can build the network tools images locally using the provided Makefile commands:

```bash
# Build single-arch images for local testing
make build-local-nettools

# Build multi-architecture images locally (doesn't push)
make build-nettools-images

# Build and push the multi-architecture images to GitHub Container Registry
make push-nettools-images
```

Before pushing to GitHub Container Registry, you need to authenticate:

1. Create a GitHub Personal Access Token with `write:packages` permission
2. Set environment variables and login:

```bash
# Set your GitHub username and token
export GITHUB_USERNAME=your-github-username
export GITHUB_TOKEN=your-personal-access-token

# Login to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin

# Run the make command with the environment variables
make push-nettools-images
```

You can also set the variables inline with the make command:

```bash
GITHUB_USERNAME=your-github-username GITHUB_TOKEN=your-personal-access-token make push-nettools-images
```

### IPTables command

```text
pumba iptables -h
NAME:
   Pumba iptables - emulate loss of incoming packets, all ports and address arguments will result in seperate rules
USAGE:
   Pumba iptables command [command options] containers (name, list of names, or RE2 regex if prefixed with "re2:"
COMMANDS:
   loss  adds iptables rules to generate packet loss on ingress traffic
OPTIONS:
   --duration value, -d value             network emulation duration; should be smaller than recurrent interval; use with optional unit suffix: 'ms/s/m/h' (default: 0s)
   --interface value, -i value            network interface to apply input rules on (default: "eth0")
   --protocol value, -p value             protocol to apply input rules on (any, udp, tcp or icmp) (default: "any")
   --source value, --src value, -s value  source IP filter; supports multiple IPs; supports CIDR notation
   --destination value, --dest value      destination IP filter; supports multiple IPs; supports CIDR notation
   --src-port value, --sport value        source port filter; supports multiple ports (comma-separated)
   --dst-port value, --dport value        destination port filter; supports multiple ports (comma-separated)
   --iptables-image value                 Docker image with iptables and tc tools (default: "ghcr.io/alexei-led/pumba-alpine-nettools:latest")
   --pull-image                           force pull iptables-image
   --help, -h                             show help
```

#### IPTables loss command

```text
pumba iptables loss -h
NAME:
   Pumba iptables loss - adds iptables rules to generate packet loss on ingress traffic
USAGE:
   Pumba iptables loss [command options] containers (name, list of names, or RE2 regex if prefixed with "re2:"
DESCRIPTION:
   adds packet losses on ingress traffic by setting iptable statistic rules
   see:  https://www.man7.org/linux/man-pages/man8/iptables-extensions.8.html
OPTIONS:
   --mode value         matching mode, supported modes are random and nth (default: "random")
   --probability value  set the probability for a packet to me matched in random mode, between 0.0 and 1.0 (default: 0)
   --every value        match one packet every nth packet, works only with nth mode (default: 0)
   --packet value       set the initial counter value (0 <= packet <= n-1, default 0) for nth mode (default: 0)
```

#### Using the `iptables` Commands

Pumba's `iptables` command allows you to simulate packet loss for incoming network traffic, with powerful filtering options. This can be
used to test application resilience to network issues.

##### Examples

```bash
# Drop 20% of incoming packets for a container named "web"
pumba iptables loss --probability 0.2 web
```

```bash
# Drop every 5th packet coming from IP 192.168.1.100 to container "api" on port 8080
pumba iptables loss --mode nth --every 5 --protocol tcp --source 192.168.1.100 --dst-port 8080 api
```

```bash
# Drop 15% of incoming ICMP packets (ping) for all containers with names matching "database"
pumba iptables loss --probability 0.15 --protocol icmp "re2:database"
```

```bash
# Complex example: Drop 25% of TCP traffic coming to port 443 from a specific subnet, for 30 seconds
pumba iptables --duration 30s --protocol tcp --source 10.0.0.0/24 --dst-port 443 \
    loss --probability 0.25 mycontainer
```

##### `iptables` Image Requirements

Pumba uses the nettools images (which include both `tc` and `iptables`) for filtering incoming network traffic.
You have two options:

1. Make sure the target container has the `iptables` tool installed
   (install the `iptables` package)

2. Use the `--iptables-image` option to specify a Docker image with
   the `iptables` tool.

   Pumba will create a helper container from this image with `NET_ADMIN`
   capability and reuse the target container's network stack.

   The recommended images are:
    - `ghcr.io/alexei-led/pumba-alpine-nettools:latest` (Alpine-based)
    - `ghcr.io/alexei-led/pumba-debian-nettools:latest` (Debian-based)

   Both images support multiple architectures (amd64, arm64).

### Advanced Network Chaos Scenarios

Pumba allows you to create complex and realistic network chaos scenarios by combining multiple network manipulation commands. This is
particularly useful for simulating real-world network conditions where multiple issues might occur simultaneously.

#### Asymmetric Network Conditions

In real networks, upload and download speeds/quality often differ. You can simulate this using a combination of `netem` for outgoing traffic
and `iptables` for incoming traffic:

```bash
# Add delay to outgoing traffic (slow uploads)
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 5m delay --time 500 myapp &

# Add packet loss to incoming traffic (unreliable downloads)
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 5m loss --probability 0.1 myapp &
```

#### Combined Network Degradation

Test how your application handles multiple concurrent network issues:

```bash
# Limit bandwidth and add packet corruption
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 10m rate --rate 1mbit myapp &

# Add packet loss to incoming traffic
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --duration 10m loss --probability 0.05 myapp &
```

#### Testing Microservices Resilience

Use Pumba to test how your microservices architecture responds to network failures between specific services:

```bash
# Add high latency between service A and service B
pumba netem --tc-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --target service-b-ip --duration 5m delay --time 2000 --jitter 500 service-a &

# Add packet loss from service B to service C
pumba iptables --iptables-image ghcr.io/alexei-led/pumba-alpine-nettools:latest \
  --source service-c-ip --duration 5m loss --probability 0.2 service-b &
```

#### Example Script

You can find a complete example script for combined chaos testing in the [examples directory](examples/pumba_combined.sh).

For detailed guidance on advanced network chaos testing scenarios, best practices, and troubleshooting, see
the [Advanced Network Chaos Testing Documentation](docs/advanced-network-chaos.md).

### Stress testing Docker containers

Pumba can inject [stress-ng](https://kernel.ubuntu.com/~cking/stress-ng/)
testing tool into a target container(s) `cgroup` and control stress test run.

```text
NAME:
   pumba stress - stress test a specified containers

USAGE:
   pumba stress [command options] containers (name, list of names, or RE2 regex if prefixed with "re2:")

DESCRIPTION:
   stress test target container(s)

OPTIONS:
   --duration value, -d value  stress duration: must be shorter than recurrent interval; use with optional unit suffix: 'ms/s/m/h'
   --stress-image value        Docker image with stress-ng tool, cgroup-bin and docker packages, and dockhack script (default: "alexeiled/stress-ng:latest-ubuntu")
   --pull-image                pull stress-image from Docker registry
   --stressors value           stress-ng stressors; see https://kernel.ubuntu.com/~cking/stress-ng/ (default: "--cpu 4 --timeout 60s")
```

#### stress-ng image requirements

Pumba uses
[alexeiled/stress-ng:latest-ubuntu](https://hub.docker.com/r/alexeiled/stress-ng/)
`stress-ng` Ubuntu-based Docker image with statically linked `stress-ng` tool.

You can provide your own image, but it must include the following tools:

1. `stress-ng` tool (in `$PATH`)
1. Bash shell
1. [`dockhack`](https://github.com/tavisrudd/dockhack) helper Bash script (in
   `$PATH`)
1. `docker` client CLI tool (runnable without `sudo`)
1. `cgexec` tool, available from `cgroups-tools` or/and `cgroup-bin` packages

### Running inside Docker container

If you choose to use Pumba Docker
[image](https://hub.docker.com/r/gaiaadm/pumba/) on Linux, use the following
command:

```text
# run 10 Docker containers named test_(index)
for i in `seq 1 10`; do docker run -d --name test_$i --rm alpine tail -f /dev/null; done

# once in a 10 seconds, try to kill (with `SIGKILL` signal) all containers named **test(something)**
# on same Docker host, where Pumba container is running

$ docker run -it --rm  -v /var/run/docker.sock:/var/run/docker.sock gaiaadm/pumba --interval=10s --random --log-level=info kill --signal=SIGKILL "re2:^test"

```

**Note:** from version `0.6` Pumba Docker image is a `scratch` Docker image,
that contains only single `pumba` binary file and `ENTRYPOINT` set to the
`pumba` command.

**Note:** For Windows and OS X you will need to use `--host` argument, since
there is no unix socket `/var/run/docker.sock` to mount.

### Running Pumba on Kubernetes cluster

If you are running Kubernetes, you can take advantage of DaemonSets to
automatically deploy the Pumba on selected K8s nodes, using `nodeSelector` or
`nodeAffinity`, see
[Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/).

You'll then be able to deploy the DaemonSet with the command:

```sh
kubectl create -f deploy/pumba_kube.yml
```

K8s automatically assigns labels to Docker container, and you can use Pumba
`--label` filter to create chaos for specific Pods and Namespaces.

K8s auto-assigned container labels, than can be used by Pumba:

```yaml
"io.kubernetes.container.name": "test-container"
"io.kubernetes.pod.name": "test-pod"
"io.kubernetes.pod.namespace": "test-namespace"
```

It's possible to run multiple Pumba commands in the same DaemonSet using
multiple Pumba containers, see `deploy/pumba_kube.yml` example.

If you are not running Kubernetes >= 1.1.0 or do not want to use DaemonSets, you
can also run the Pumba as a regular docker container on each node you want to
make chaos (see above)

**Note:** running `pumba netem` commands on minikube clusters will not work,
because the sch_netem kernel module is missing in the minikube VM!

## Build instructions

You can build Pumba with or without Go installed on your machine.

### Build using local Go environment

In order to build Pumba, you need to have Go 1.6+ setup on your machine.

Here is the approximate list of commands you will need to run:

```sh
# create required folder
cd $GOPATH
mkdir github.com/alexei-led && cd github.com/alexei-led

# clone pumba
git clone git@github.com:alexei-led/pumba.git
cd pumba

# build pumba binary
make

# run tests and create HTML coverage report
make test-coverage

# create pumba binaries for multiple platforms
make release
```

### Build using Docker

You do not have to install and configure Go in order to build and test Pumba
project.
Pumba uses Docker multistage build to create final tiny Docker image.

First of all clone Pumba git repository:

```sh
git clone git@github.com:alexei-led/pumba.git
cd pumba
```

Now create a new Pumba Docker image.

```sh
DOCKER_BUILDKIT=1 docker build -t pumba -f docker/Dockerfile .
```

## License

Code is under the
[Apache License v2](https://www.apache.org/licenses/LICENSE-2.0.txt).
