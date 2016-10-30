#!/bin/bash
[ -z "$DIST" ] && DIST=.dist
exarch="amd64 386"
oslist="linux windows darwin"
BUILD_DATE=$(date -u '+%Y/%m/%d')
BUILD_VERSION=$(git describe --tags)
CGO_ENABLE=0

gox_build() {
  [ -d "${DIST}" ] && rm -rf "${DIST}/*"
  [ -d "${DIST}" ] || mkdir -p "${DIST}"
  echo "Building" ${BUILD_VERSION} "on" ${BUILD_DATE}
  glide install
  gox -os="${oslist}" -arch="${exarch}"  -ldflags "-X main.Version=${BUILD_VERSION} -X main.BuildDate='${BUILD_DATE}'" -verbose -output="${DIST}/pumba_{{.OS}}_{{.Arch}}"
}

gox_build
