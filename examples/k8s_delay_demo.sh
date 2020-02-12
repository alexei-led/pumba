#!/bin/sh

set -o xtrace

kubectl run test-2 --rm -it --restart=Never --image alpine -- ping 1.1.1.1