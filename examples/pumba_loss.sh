#!/bin/sh

set -o xtrace

pumba netem loss --help
read -p "Press enter to continue"

pumba --log-level=info netem --duration 20s --tc-image gaiadocker/iproute2 loss --percent 20 client