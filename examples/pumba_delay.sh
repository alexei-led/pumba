#!/bin/sh

set -o xtrace

pumba netem delay --help
read -p "Press enter to continue"

pumba --log-level=info --interval=20s netem --tc-image=ghcr.io/alexei-led/pumba/pumba-debian-nettools --duration=10s delay --time=3000 --jitter=20 ping
