#!/bin/sh

# Run test containers via Podman. Pair with pumba_podman_kill.sh.
set -o xtrace

trap ctrl_c INT

ctrl_c() {
	for i in $(seq 1 10); do
		podman rm -f "killme-${i}"
	done
}

for i in $(seq 1 10); do
	podman run -d --name "killme-${i}" alpine tail -f /dev/null
done

watch podman ps -a
