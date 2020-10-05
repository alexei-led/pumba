#!/bin/sh

set -o xtrace

pumba kill --help
read -p "Press enter to continue"

pumba --log-level=info --interval=3s --random kill 're2:^killme*'
