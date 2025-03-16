#!/bin/sh

set -o xtrace

pumba netem loss --help
read -p "Press enter to continue"

pumba --log-level=info netem --duration 20s --tc-image ghcr.io/alexei-led/pumba-debian-nettools --percent 20 client