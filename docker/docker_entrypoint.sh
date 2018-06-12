#!/bin/sh
set -e

if [ "$1" = "pumba" ]; then
  PUMBA_UID=9001
  DOCKER_SOCKET=/var/run/docker.sock
  DOCKER_GROUP=docker
  echo "currently $(id) switch to ${PUMBA_UID}"
  if [ -S ${DOCKER_SOCKET} ]; then
    if ! id -u ${PUMBA_UID} > /dev/null 2>&1; then
      echo "adding pumba use for uid ${PUMBA_UID}"
      adduser -u ${PUMBA_UID} -D -S pumba
    fi
    DOCKER_GID=$(stat -c '%g' ${DOCKER_SOCKET})
    echo "adding ${DOCKER_GID} to group ${DOCKER_GROUP}"
    groupadd -for -g ${DOCKER_GID} ${DOCKER_GROUP}
    usermod -aG ${DOCKER_GROUP} pumba
  fi
  exec su-exec ${PUMBA_UID} "$@"
fi

exec "$@"
