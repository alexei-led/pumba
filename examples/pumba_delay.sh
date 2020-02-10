#!/bin/sh

set -o xtrace

pumba netem delay --help
read -p "Press enter to continue"

pumba --log-level=info --interval=20s netem --tc-image=gaiadocker/iproute2 --duration=10s delay --time=3000 --jitter=20 ping
