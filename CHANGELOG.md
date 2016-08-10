# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [v0.2.4] - 2016-08-10
### Added
- `netem loss` network emulation packer loss, based on independent (Bernoulli) probability model
- `netem loss-state` network emulation packer loss, based on 4-Markov state probability model
- `netem loss-gemodel` network emulation packer loss, according to the Gilbert-Elliot loss model

## [v0.2.3] - 2016-08-07
### Fixed
- `pause` command now can be interrupted with `Ctrl-C`; all paused processes will be unpaused
- BUG: when using single container name, pumba disturbs all containers

## [v0.2.2] - 2016-08-04
### Changed
- `--interval` flag is optional now; if missing Pumba will do single chaos action and exit
- `netem delay --distribution` new option to define optional delay distribution, can be: {uniform | normal | pareto |  paretonormal}
- check added to verify `tc` tool existence in container (`tc` is required for network emulation; part of `iproute2` package)
- Use `Ctrl-C` to abort Pumba execution at any time

## [v0.2.0] - 2016-07-20
### Added
- Network emulation for egress container traffic, powered by [netem](http://www.linuxfoundation.org/collaborate/workgroups/networking/netem)
### Updated
- **Breaking Change** command line simplification ...
- `chaos` command had been replaced by multiple standalone commands: `kill`, `netem`, `pause`, `rm`, `stop`
- now it's possible to run multiple Pumba Docker containers (do not prevent)
- **Only ONE** command per single Pumba run is supported, but it's possible to run multiple Pumba processes and containers

## [v0.1.11] - 2016-06-27
### Added
- pause container processes for specified interval

## [v0.1.10] - 2016-06-05
### Fixed
- set proper release tag in GitHub

## [0.1.9] - 2016-05-22
### Fixed
- speed up build

## [0.1.8] - 2016-05-22
### Fixed
- Added CA ca-certificates to Docker image: required for HTTPS

## [0.1.7] - 2016-05-21
### Added
### Fixed
- Report Pumba events to Slack

## [0.1.6] - 2016-04-25
### Added
- added `gosu` to Pumba Docker image
- Use Docker Label `com.gaiaadm.pumba.skip` to make Pumba ignoring the container. Avoid abusing it though.
### Fixed
- Pumba runs as `pumba:pumba` user, instead of `root`

## [0.1.5] - 2016-04-13
### Added
- File: `pumba_kube.yml` Kubernetes (1.1.x) deployment manifest
- File: `pumba_coreos.service` CoreOS `fleet` service file
- Flag: `--json` flag. When specified log will be generated in JSON format (Logstash and Splunk friendly)
- Flag: `--slackhook` Slack web hook URL. Now Pumba can report log events to specified Slack channel.
- Flag: `--slackchannel` Slack channel to report Pumba events in.
- Flag: `--dry` enable 'dry run' mode: do not 'kill' containers, just log intention
### Changed
- by default produce colarful log to TTY
### Fixed
- fix failure when container name is empty (all containers)

## [0.1.4] - 2016-04-08
This is initial release of Pumba Docker Chaos Testing
### Added
- `run` command
- `--random` option: randomly select matching image to "kill"
