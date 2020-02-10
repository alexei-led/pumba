#!/bin/sh

set -o xtrace

pumba stop --help
read -p "Press enter to continue"

pumba --log-level=info --interval=15s --random stop --duration=7s --restart 're2:^stopme*'
