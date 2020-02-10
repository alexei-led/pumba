#!/bin/sh

set -o xtrace

for i in $(seq 1 10); do docker run -d --name=stopme-${i} alpine tail -f /dev/null; done

watch docker ps -a

for i in $(seq 1 10); do docker kill stopme-${i}; done
