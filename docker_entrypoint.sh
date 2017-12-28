#!/bin/sh
set -e

if [ "$1" = "pumba" ]; then
  if [ -S /var/run/docker.sock ]; then
    chown -R pumba:pumba /var/run/docker.sock
  fi
  exec su-exec pumba:pumba "$@"
fi

exec "$@"
