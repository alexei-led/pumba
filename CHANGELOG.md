# Changelog

## [0.7.5](https://github.com/alexei-led/pumba/tree/0.7.5) (2020-09-12)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.7.4...0.7.5)

**Implemented enhancements:**

- Pumba Commands for chaos testing [\#89](https://github.com/alexei-led/pumba/issues/89)
- Kubernetes Chaos Test [\#51](https://github.com/alexei-led/pumba/issues/51)

**Closed issues:**

- Build instructions are not correct for 0.7.4 [\#170](https://github.com/alexei-led/pumba/issues/170)
- TC not found or failed [\#159](https://github.com/alexei-led/pumba/issues/159)
- NetEm multiple targets ip's leads to fatal execption, unrecognized CIDR [\#158](https://github.com/alexei-led/pumba/issues/158)
- pumba wildcard not working properly [\#133](https://github.com/alexei-led/pumba/issues/133)
- Documentation doubts [\#119](https://github.com/alexei-led/pumba/issues/119)

**Merged pull requests:**

- Skip error [\#177](https://github.com/alexei-led/pumba/pull/177) ([alexei-led](https://github.com/alexei-led))
- Linter fix [\#176](https://github.com/alexei-led/pumba/pull/176) ([alexei-led](https://github.com/alexei-led))

## [0.7.4](https://github.com/alexei-led/pumba/tree/0.7.4) (2020-07-20)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.7.3...0.7.4)

**Closed issues:**

- Netem target port filter [\#130](https://github.com/alexei-led/pumba/issues/130)

**Merged pull requests:**

- Adding port targeting [\#163](https://github.com/alexei-led/pumba/pull/163) ([chuckkQ](https://github.com/chuckkQ))

## [0.7.3](https://github.com/alexei-led/pumba/tree/0.7.3) (2020-07-19)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.7.2...0.7.3)

**Implemented enhancements:**

- Attacking a container's available cpu/ram? [\#114](https://github.com/alexei-led/pumba/issues/114)
- Restrict pumba targets by docker labels [\#86](https://github.com/alexei-led/pumba/issues/86)
- stress Docker host [\#50](https://github.com/alexei-led/pumba/issues/50)

**Closed issues:**

- Error running Pumba on OpenShift version 4.3 [\#169](https://github.com/alexei-led/pumba/issues/169)
- Pumba docker container is exiting as soon as container is initiated [\#165](https://github.com/alexei-led/pumba/issues/165)
- unexpected behaviour of --interval flag [\#162](https://github.com/alexei-led/pumba/issues/162)
- Unable to start Pumba docker container on MacOs [\#161](https://github.com/alexei-led/pumba/issues/161)
- Control the packet selection in loss/corruption/duplication commands [\#160](https://github.com/alexei-led/pumba/issues/160)
- Network emulation not working on armv7 containers [\#156](https://github.com/alexei-led/pumba/issues/156)
- Using Pumba with Docker desktop on Windows won't randomly kill a container  [\#155](https://github.com/alexei-led/pumba/issues/155)
- How to use Pumba on AWS Fargate Containers [\#154](https://github.com/alexei-led/pumba/issues/154)
- Running stress-ng with K8s on an alpine based image causes errors [\#153](https://github.com/alexei-led/pumba/issues/153)
- Why use a DaemonSet in Kubernetes? [\#150](https://github.com/alexei-led/pumba/issues/150)
- Error when running 'pumba stress' [\#149](https://github.com/alexei-led/pumba/issues/149)
- regression: with --tc-image flag, iproute2 sidecar container is not deleted after the command execution finishes [\#135](https://github.com/alexei-led/pumba/issues/135)
- https://goo.gl/SUKo6T sunset [\#81](https://github.com/alexei-led/pumba/issues/81)
- Is there a Pumba API? [\#75](https://github.com/alexei-led/pumba/issues/75)

**Merged pull requests:**

- Update docker engine to allow cleanup [\#167](https://github.com/alexei-led/pumba/pull/167) ([chuckkQ](https://github.com/chuckkQ))

## [0.7.2](https://github.com/alexei-led/pumba/tree/0.7.2) (2020-02-26)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.7.1...0.7.2)

**Merged pull requests:**

- Refactor log errors [\#152](https://github.com/alexei-led/pumba/pull/152) ([alexei-led](https://github.com/alexei-led))

## [0.7.1](https://github.com/alexei-led/pumba/tree/0.7.1) (2020-02-10)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.7.0...0.7.1)

**Implemented enhancements:**

- Anyway to run this on a Raspberry PI [\#146](https://github.com/alexei-led/pumba/issues/146)

## [0.7.0](https://github.com/alexei-led/pumba/tree/0.7.0) (2020-02-09)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.8...0.7.0)

**Closed issues:**

- pumba netem is affecting all containers in host and not responding to kill -9 when running in background [\#147](https://github.com/alexei-led/pumba/issues/147)
- docker client version too new [\#144](https://github.com/alexei-led/pumba/issues/144)

**Merged pull requests:**

- stress test target container [\#148](https://github.com/alexei-led/pumba/pull/148) ([alexei-led](https://github.com/alexei-led))

## [0.6.8](https://github.com/alexei-led/pumba/tree/0.6.8) (2019-12-21)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.7...0.6.8)

## [0.6.7](https://github.com/alexei-led/pumba/tree/0.6.7) (2019-12-19)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.5...0.6.7)

**Closed issues:**

- Bug: netem command not working on minikube [\#140](https://github.com/alexei-led/pumba/issues/140)
- DaemonSet apiVersion changed with newer Kubernetes version [\#138](https://github.com/alexei-led/pumba/issues/138)

**Merged pull requests:**

- Use container labels as additional filter [\#143](https://github.com/alexei-led/pumba/pull/143) ([alexei-led](https://github.com/alexei-led))
- Add minikube netem hint [\#142](https://github.com/alexei-led/pumba/pull/142) ([LaumiH](https://github.com/LaumiH))
- Fix daemonset apiVersion and nodeSelector for k8s \>= 1.16.2 [\#139](https://github.com/alexei-led/pumba/pull/139) ([LaumiH](https://github.com/LaumiH))

## [0.6.5](https://github.com/alexei-led/pumba/tree/0.6.5) (2019-10-01)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.4...0.6.5)

**Closed issues:**

- pull-image can only be true [\#132](https://github.com/alexei-led/pumba/issues/132)
- K8s & netem question [\#128](https://github.com/alexei-led/pumba/issues/128)

**Merged pull requests:**

- Use GitHub Actions [\#136](https://github.com/alexei-led/pumba/pull/136) ([alexei-led](https://github.com/alexei-led))

## [0.6.4](https://github.com/alexei-led/pumba/tree/0.6.4) (2019-05-02)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.3...0.6.4)

**Fixed bugs:**

- Unable to specify target network, only single host IP [\#126](https://github.com/alexei-led/pumba/issues/126)

**Closed issues:**

- How to add more containers to kill [\#124](https://github.com/alexei-led/pumba/issues/124)
- Using re2 in pumba command in kubernetes template [\#123](https://github.com/alexei-led/pumba/issues/123)
- Fedora - command 'tc' not found [\#121](https://github.com/alexei-led/pumba/issues/121)
- log-level behaviour [\#120](https://github.com/alexei-led/pumba/issues/120)

**Merged pull requests:**

- Add support for specifying target networks \(CIDR notation\) \#126 [\#127](https://github.com/alexei-led/pumba/pull/127) ([gmpify](https://github.com/gmpify))

## [0.6.3](https://github.com/alexei-led/pumba/tree/0.6.3) (2019-03-12)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.2...0.6.3)

**Fixed bugs:**

- Regex issue with interface name [\#109](https://github.com/alexei-led/pumba/issues/109)

**Closed issues:**

- Slack hook certificate error [\#122](https://github.com/alexei-led/pumba/issues/122)

**Merged pull requests:**

- Fix spelling error [\#116](https://github.com/alexei-led/pumba/pull/116) ([CatEars](https://github.com/CatEars))
- Update interface regexp [\#108](https://github.com/alexei-led/pumba/pull/108) ([ddliu](https://github.com/ddliu))

## [0.6.2](https://github.com/alexei-led/pumba/tree/0.6.2) (2018-12-12)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.1...0.6.2)

**Implemented enhancements:**

- Deployment failed on Kubernetes 1.7 [\#42](https://github.com/alexei-led/pumba/issues/42)

**Fixed bugs:**

- Multiple matching container `netem` commands executed in sequentially when triggered with `docker run` [\#112](https://github.com/alexei-led/pumba/issues/112)

**Closed issues:**

- Pumba daemonset pods are crashing [\#110](https://github.com/alexei-led/pumba/issues/110)

**Merged pull requests:**

- run netem in parallel on multiple containers. fix \#112 [\#113](https://github.com/alexei-led/pumba/pull/113) ([alexei-led](https://github.com/alexei-led))
- Change command to args and support entrypoint [\#111](https://github.com/alexei-led/pumba/pull/111) ([yaron-idan](https://github.com/yaron-idan))

## [0.6.1](https://github.com/alexei-led/pumba/tree/0.6.1) (2018-11-15)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.6.0...0.6.1)

**Implemented enhancements:**

- Better killing and respawing options  [\#46](https://github.com/alexei-led/pumba/issues/46)
- No such image: gaiadocker/iproute2 [\#40](https://github.com/alexei-led/pumba/issues/40)

**Fixed bugs:**

- reuse tc container [\#97](https://github.com/alexei-led/pumba/issues/97)

**Closed issues:**

- when using --tc-image flag the sidekick image is not deleted after the command execution finishes [\#106](https://github.com/alexei-led/pumba/issues/106)
- Cannot connect to the Docker daemon [\#105](https://github.com/alexei-led/pumba/issues/105)
- Strange latency spikes on 4.15.0-36 kernel [\#103](https://github.com/alexei-led/pumba/issues/103)

## [0.6.0](https://github.com/alexei-led/pumba/tree/0.6.0) (2018-10-08)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.5.2...0.6.0)

**Closed issues:**

- Any Plan about Network Partitioning Simulation? [\#96](https://github.com/alexei-led/pumba/issues/96)
- Add SVG version of Pumba logo [\#94](https://github.com/alexei-led/pumba/issues/94)

**Merged pull requests:**

- Fixing tc image and more [\#107](https://github.com/alexei-led/pumba/pull/107) ([alexei-led](https://github.com/alexei-led))
- use SCRATCH image for base image [\#101](https://github.com/alexei-led/pumba/pull/101) ([alexei-led](https://github.com/alexei-led))
- Better support for CI tool and Codecov [\#100](https://github.com/alexei-led/pumba/pull/100) ([alexei-led](https://github.com/alexei-led))
- Refactor: Initialize CLI Commands in a separate func. [\#99](https://github.com/alexei-led/pumba/pull/99) ([nawazish-github](https://github.com/nawazish-github))

## [0.5.2](https://github.com/alexei-led/pumba/tree/0.5.2) (2018-09-03)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.5.0...0.5.2)

**Implemented enhancements:**

- Pumba is not an importable package [\#60](https://github.com/alexei-led/pumba/issues/60)
- Add Start command. [\#59](https://github.com/alexei-led/pumba/issues/59)

**Fixed bugs:**

- Got permission denied after using pumba to delay network [\#83](https://github.com/alexei-led/pumba/issues/83)
- docker\_entrypoint.sh changes ownership of parent socket [\#38](https://github.com/alexei-led/pumba/issues/38)

**Closed issues:**

- Pumba attack - visualize the execution steps in command terminal  [\#91](https://github.com/alexei-led/pumba/issues/91)
- Pumba run time startup issues [\#88](https://github.com/alexei-led/pumba/issues/88)
- cat: can't open 'VERSION': No such file or directory [\#87](https://github.com/alexei-led/pumba/issues/87)
- netem delay loses the first 3 packets  [\#72](https://github.com/alexei-led/pumba/issues/72)
- Pumba container exiting without any error [\#70](https://github.com/alexei-led/pumba/issues/70)

**Merged pull requests:**

- Add corrupt and duplicate netem commands [\#95](https://github.com/alexei-led/pumba/pull/95) ([philipgloyne](https://github.com/philipgloyne))

## [0.5.0](https://github.com/alexei-led/pumba/tree/0.5.0) (2018-05-21)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.8...0.5.0)

**Closed issues:**

- Slack hooks fail due to no ca [\#84](https://github.com/alexei-led/pumba/issues/84)

**Merged pull requests:**

- Code refactoring [\#85](https://github.com/alexei-led/pumba/pull/85) ([alexei-led](https://github.com/alexei-led))
- implement 'contains' in a cheaper, simpler way [\#82](https://github.com/alexei-led/pumba/pull/82) ([Dieterbe](https://github.com/Dieterbe))
- Spring cleanup [\#80](https://github.com/alexei-led/pumba/pull/80) ([alexei-led](https://github.com/alexei-led))

## [0.4.8](https://github.com/alexei-led/pumba/tree/0.4.8) (2018-03-12)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.7...0.4.8)

**Implemented enhancements:**

- Fix `netem` when destination IP filter is defined [\#52](https://github.com/alexei-led/pumba/issues/52)

**Fixed bugs:**

- netem command fails on images where user != root [\#43](https://github.com/alexei-led/pumba/issues/43)

**Closed issues:**

- use dumb-init [\#69](https://github.com/alexei-led/pumba/issues/69)
- use su-exec instead of gosu [\#68](https://github.com/alexei-led/pumba/issues/68)
- kubernetes command should be an array and not a string [\#63](https://github.com/alexei-led/pumba/issues/63)
- custom built container kills itself [\#62](https://github.com/alexei-led/pumba/issues/62)
- suggest kubernetes limits and requests [\#61](https://github.com/alexei-led/pumba/issues/61)
- allow targetting multiple specific ip's [\#57](https://github.com/alexei-led/pumba/issues/57)

**Merged pull requests:**

- moving git repo to alexei-led [\#78](https://github.com/alexei-led/pumba/pull/78) ([alexei-led](https://github.com/alexei-led))
- Limit the number of container to kill \#46 [\#77](https://github.com/alexei-led/pumba/pull/77) ([camilocot](https://github.com/camilocot))
- Add Start command. \#59 [\#76](https://github.com/alexei-led/pumba/pull/76) ([camilocot](https://github.com/camilocot))
- very minor min corrections [\#74](https://github.com/alexei-led/pumba/pull/74) ([lazerion](https://github.com/lazerion))
- use dumb-init and su-exec [\#71](https://github.com/alexei-led/pumba/pull/71) ([grosser](https://github.com/grosser))
- add requests/limits so container does not be come too greedy [\#67](https://github.com/alexei-led/pumba/pull/67) ([grosser](https://github.com/grosser))
- avoid self-killing on kubernetes [\#66](https://github.com/alexei-led/pumba/pull/66) ([grosser](https://github.com/grosser))
- prefer regular nodes by default [\#65](https://github.com/alexei-led/pumba/pull/65) ([grosser](https://github.com/grosser))
- do not spam extra shell / make killing soft by default [\#64](https://github.com/alexei-led/pumba/pull/64) ([grosser](https://github.com/grosser))
- support specifying multiple target IP's [\#58](https://github.com/alexei-led/pumba/pull/58) ([Dieterbe](https://github.com/Dieterbe))
- fix logging of configs [\#56](https://github.com/alexei-led/pumba/pull/56) ([Dieterbe](https://github.com/Dieterbe))

## [0.4.7](https://github.com/alexei-led/pumba/tree/0.4.7) (2017-11-14)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.6...0.4.7)

**Fixed bugs:**

- Pumba does not seem to work in my environment [\#33](https://github.com/alexei-led/pumba/issues/33)

**Merged pull requests:**

- Fixes [\#55](https://github.com/alexei-led/pumba/pull/55) ([Dieterbe](https://github.com/Dieterbe))
- fix typo's [\#54](https://github.com/alexei-led/pumba/pull/54) ([Dieterbe](https://github.com/Dieterbe))

## [0.4.6](https://github.com/alexei-led/pumba/tree/0.4.6) (2017-10-26)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.5...0.4.6)

**Implemented enhancements:**

- Pumba interact with all containers inside docker [\#41](https://github.com/alexei-led/pumba/issues/41)

**Fixed bugs:**

- Target IP filter blocking all traffic [\#39](https://github.com/alexei-led/pumba/issues/39)

**Closed issues:**

- Regex not working [\#47](https://github.com/alexei-led/pumba/issues/47)
- Building Error - "golang:1.8-alpine AS builder" [\#45](https://github.com/alexei-led/pumba/issues/45)

**Merged pull requests:**

- Add a Gitter chat badge to README.md [\#49](https://github.com/alexei-led/pumba/pull/49) ([gitter-badger](https://github.com/gitter-badger))
- Creates a deploy file for OpenShift [\#48](https://github.com/alexei-led/pumba/pull/48) ([lordofthejars](https://github.com/lordofthejars))

## [0.4.5](https://github.com/alexei-led/pumba/tree/0.4.5) (2017-09-06)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.4...0.4.5)

**Fixed bugs:**

- not work in k8s ver 1.3 [\#19](https://github.com/alexei-led/pumba/issues/19)

## [0.4.4](https://github.com/alexei-led/pumba/tree/0.4.4) (2017-07-08)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.3...0.4.4)

## [0.4.3](https://github.com/alexei-led/pumba/tree/0.4.3) (2017-07-07)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.2...0.4.3)

**Implemented enhancements:**

- tc command check [\#35](https://github.com/alexei-led/pumba/issues/35)

**Fixed bugs:**

- Cannot remove running container [\#31](https://github.com/alexei-led/pumba/issues/31)
- "pumba rm" without "--force" flag is useless [\#30](https://github.com/alexei-led/pumba/issues/30)

**Closed issues:**

- Replace `samalba/dockerclient` library [\#14](https://github.com/alexei-led/pumba/issues/14)

## [0.4.2](https://github.com/alexei-led/pumba/tree/0.4.2) (2017-03-16)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.1...0.4.2)

**Merged pull requests:**

- Added basic e2e tests [\#37](https://github.com/alexei-led/pumba/pull/37) ([slnowak](https://github.com/slnowak))
- Pumba is now able to remove container [\#34](https://github.com/alexei-led/pumba/pull/34) ([slnowak](https://github.com/slnowak))

## [0.4.1](https://github.com/alexei-led/pumba/tree/0.4.1) (2017-02-01)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.4.0-2-gdf5e4a3...0.4.1)

## [0.4.0-2-gdf5e4a3](https://github.com/alexei-led/pumba/tree/0.4.0-2-gdf5e4a3) (2017-01-29)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.3.2...0.4.0-2-gdf5e4a3)

**Merged pull requests:**

- Get rid of samalba client [\#32](https://github.com/alexei-led/pumba/pull/32) ([slnowak](https://github.com/slnowak))

## [0.3.2](https://github.com/alexei-led/pumba/tree/0.3.2) (2017-01-17)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.3.1...0.3.2)

## [0.3.1](https://github.com/alexei-led/pumba/tree/0.3.1) (2016-12-13)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.3.0...0.3.1)

**Implemented enhancements:**

- Implement `rate` bandwidth limit [\#25](https://github.com/alexei-led/pumba/issues/25)

**Closed issues:**

- Debug messages problem [\#28](https://github.com/alexei-led/pumba/issues/28)

**Merged pull requests:**

- Implement rate bandwidth limit [\#29](https://github.com/alexei-led/pumba/pull/29) ([meqif](https://github.com/meqif))

## [0.3.0](https://github.com/alexei-led/pumba/tree/0.3.0) (2016-11-24)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.9-4257dcf...0.3.0)

**Closed issues:**

- Unable to start the pumba container [\#27](https://github.com/alexei-led/pumba/issues/27)

## [0.2.9-4257dcf](https://github.com/alexei-led/pumba/tree/0.2.9-4257dcf) (2016-10-28)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.9...0.2.9-4257dcf)

## [0.2.9](https://github.com/alexei-led/pumba/tree/0.2.9) (2016-10-28)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.8...0.2.9)

## [0.2.8](https://github.com/alexei-led/pumba/tree/0.2.8) (2016-10-28)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.7...0.2.8)

## [0.2.7](https://github.com/alexei-led/pumba/tree/0.2.7) (2016-10-27)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.6-3-g705f13b...0.2.7)

## [0.2.6-3-g705f13b](https://github.com/alexei-led/pumba/tree/0.2.6-3-g705f13b) (2016-10-25)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.6...0.2.6-3-g705f13b)

**Implemented enhancements:**

- One time run w/o interval [\#20](https://github.com/alexei-led/pumba/issues/20)
- Run first action before interval [\#17](https://github.com/alexei-led/pumba/issues/17)
- Can't rely on the Docker restart policy [\#11](https://github.com/alexei-led/pumba/issues/11)

**Fixed bugs:**

- netem: add check for `iptools2` install [\#21](https://github.com/alexei-led/pumba/issues/21)

**Closed issues:**

- Chaos state [\#18](https://github.com/alexei-led/pumba/issues/18)

**Merged pull requests:**

- Fix typo: dealy -\> delay [\#26](https://github.com/alexei-led/pumba/pull/26) ([kane-c](https://github.com/kane-c))

## [0.2.6](https://github.com/alexei-led/pumba/tree/0.2.6) (2016-09-25)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.5...0.2.6)

## [0.2.5](https://github.com/alexei-led/pumba/tree/0.2.5) (2016-09-08)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.4...0.2.5)

## [0.2.4](https://github.com/alexei-led/pumba/tree/0.2.4) (2016-08-10)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.3...0.2.4)

## [0.2.3](https://github.com/alexei-led/pumba/tree/0.2.3) (2016-08-07)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.2...0.2.3)

## [0.2.2](https://github.com/alexei-led/pumba/tree/0.2.2) (2016-08-06)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.1...0.2.2)

**Implemented enhancements:**

- Disconnect container from Docker network [\#13](https://github.com/alexei-led/pumba/issues/13)
- Pause running container [\#12](https://github.com/alexei-led/pumba/issues/12)

**Closed issues:**

- Support recovery "validation" scripts [\#5](https://github.com/alexei-led/pumba/issues/5)
- Support additional Docker commands [\#4](https://github.com/alexei-led/pumba/issues/4)

## [0.2.1](https://github.com/alexei-led/pumba/tree/0.2.1) (2016-07-28)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.2.0...0.2.1)

## [0.2.0](https://github.com/alexei-led/pumba/tree/0.2.0) (2016-07-27)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.11...0.2.0)

**Merged pull requests:**

- Add basic capability to disrupt container network [\#16](https://github.com/alexei-led/pumba/pull/16) ([inbarshani](https://github.com/inbarshani))

## [0.1.11](https://github.com/alexei-led/pumba/tree/0.1.11) (2016-07-16)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.10...0.1.11)

**Closed issues:**

- Replace Gox [\#10](https://github.com/alexei-led/pumba/issues/10)
- Add a pkg installer for Mac OS X [\#9](https://github.com/alexei-led/pumba/issues/9)
- Collect container "lifecycle" activities from Docker host, Pumba is running on [\#3](https://github.com/alexei-led/pumba/issues/3)

## [0.1.10](https://github.com/alexei-led/pumba/tree/0.1.10) (2016-06-05)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.9...0.1.10)

## [0.1.9](https://github.com/alexei-led/pumba/tree/0.1.9) (2016-05-22)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.8...0.1.9)

## [0.1.8](https://github.com/alexei-led/pumba/tree/0.1.8) (2016-05-22)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.7...0.1.8)

## [0.1.7](https://github.com/alexei-led/pumba/tree/0.1.7) (2016-05-21)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.6...0.1.7)

**Closed issues:**

- Add label to skip Pumba eyes [\#8](https://github.com/alexei-led/pumba/issues/8)
- Post to Slack does not work [\#7](https://github.com/alexei-led/pumba/issues/7)

## [0.1.6](https://github.com/alexei-led/pumba/tree/0.1.6) (2016-04-25)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.5...0.1.6)

**Closed issues:**

- Are you planning to support Kubernetes or OpenSHift ? [\#6](https://github.com/alexei-led/pumba/issues/6)
- Log Pumba "kill" activities with more details about affected containers [\#2](https://github.com/alexei-led/pumba/issues/2)

## [0.1.5](https://github.com/alexei-led/pumba/tree/0.1.5) (2016-04-13)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.4...0.1.5)

**Merged pull requests:**

- Add a Bitdeli Badge to README [\#1](https://github.com/alexei-led/pumba/pull/1) ([bitdeli-chef](https://github.com/bitdeli-chef))

## [0.1.4](https://github.com/alexei-led/pumba/tree/0.1.4) (2016-04-08)

[Full Changelog](https://github.com/alexei-led/pumba/compare/0.1.3...0.1.4)

## [0.1.3](https://github.com/alexei-led/pumba/tree/0.1.3) (2016-04-04)

[Full Changelog](https://github.com/alexei-led/pumba/compare/9e876ae5807d4c3d7a859952bd8210b737a1d097...0.1.3)



\* *This Changelog was automatically generated by [github_changelog_generator](https://github.com/github-changelog-generator/github-changelog-generator)*
