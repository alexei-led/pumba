#!/bin/bash
[ -z "$DIST" ] && DIST=.dist

[ -z "$VERSION" ] && VERSION=$(cat VERSION)
[ -z "$BUILDTIME" ] && BUILDTIME=$(TZ=GMT date "+%Y-%m-%d_%H:%M_GMT")
[ -z "$GITCOMMIT" ] && GITCOMMIT=$(git rev-parse HEAD --short 2>/dev/null)
[ -z "$GITBRANCH" ] && GITBRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

exarch="amd64 386"
oslist="linux windows darwin"

gox_build() {
  [ -d "${DIST}" ] && rm -rf "${DIST}/*"
  [ -d "${DIST}" ] || mkdir -p "${DIST}"
  echo "Building" ${BUILD_VERSION} "on" ${BUILD_DATE}
  glide install -v
  gox -os="${oslist}" -arch="${exarch}" -cgo=false \
    -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GITCOMMIT} -X main.GitBranch=${GITBRANCH} -X main.BuildTime=${BUILDTIME}" \
    -verbose -output="${DIST}/pumba_{{.OS}}_{{.Arch}}" .
}

gox_build
