#!/bin/sh

set -o xtrace

docker run -it --rm --name ping  alpine ping 1.1.1.1
