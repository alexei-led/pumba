#!/bin/sh

set -o xtrace

pumba pause --help
read -p "Press enter to continue"

pumba --log-level=info --interval=15s pause --duration=10s  testme