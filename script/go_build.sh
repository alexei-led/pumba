#!/bin/bash
[ -z "$DIST" ] && DIST=.dist
CGO_ENABLE=0

go_build() {
  [ -d "${DIST}" ] && rm -rf "${DIST}/*"
  [ -d "${DIST}" ] || mkdir -p "${DIST}"
  glide install
  go build -v -o "${DIST}/pumba"
}

go_build
