# Pumba Demo

## Stop/Restart random container

1. Split screen horizontally
1. Run 10 Docker containers in bottom screen: `./stop_demo.sh`
1. Run `pumba` stopping and restarting random container: `./pumba_stop.sh`

## Pause/Resume Docker container

1. Split screen horizontally
1. Run test container printing time every second: `./pause_demo.sh`
1. Run `pumba` to pause/resume main process: `./pumba_pause.sh`

## Delay network traffic

1. Split screen horizontally
1. Run "ping" container pinging `1.1.1.1`: `./delay_demo.sh`
1. Run `pumba` adding `3000ms ± 20` delay to the "ping" container: `./pumba_delay.sh`

## Add packet loss to egress traffic

1. Split screen horizontally
1. Split bottom screen vertically
1. On the right bottom screen run UDP server: `./loss_demo_server.sh`
1. On the left bottom screen run UDP client: `./loss_demo_client.sh`; send datagrams to the UDP server
1. Run `pumba` adding packet loss to client egress traffic: `./pumba_loss.sh`

## Stress-testing container

1. Split screen horizontally
2. Run test container and show Docker stats: `./stress_demo.sh`

## Kubernetes demo: delay and pause

1. Split screen horizontally
1. Split bottom screen vertically
1. On the left bottom screen, run Pod in interactive mode: `./k8s_pause_demo.sh` - Pod prints time every second
1. On the right bottom screen, run Pod in interactive mode: `./k8s_delay_demo.sh` - Pod pings `1.1.1.1`
1. On the top screen, deploy `pumba` DaemonSet with two commands running `pause` and `delay`

## Combined tc and iptables demo: Asymmetric network degradation

This demo shows how to create a more realistic network chaos scenario by combining both outgoing and incoming traffic manipulation:

1. Split screen horizontally
1. Split bottom screen vertically
1. On the left bottom screen run a web server container: `./combined_demo_server.sh`
1. On the right bottom screen run a client container: `./combined_demo_client.sh`; send requests to the server
1. On the top screen, run Pumba with both netem and iptables commands: `./pumba_combined.sh`

This demonstrates how to simulate asymmetric network conditions (like a slow download but fast upload) that more closely resemble real-world network conditions.