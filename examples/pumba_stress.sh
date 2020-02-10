#!/bin/sh

set -o xtrace

pumba stress --help
read -p "Press enter to continue"

pumba --log-level=debug stress --duration=30s --stressors="--cpu 4 --vm 2 --vm-bytes 1G" testme