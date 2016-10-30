#!/bin/bash
[ -z "$DIST" ] && DIST=.dist

go_build() {
  [ -d "${DIST}" ] && rm -rf "${DIST}/*"
  [ -d "${DIST}" ] || mkdir -p "${DIST}"
  glide install
  CGO_ENABLED=0 go build -v -o "${DIST}/pumba"
}

go_build
