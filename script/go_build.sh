#!/bin/bash
[ -z "$DIST" ] && DIST=.dist

[ -z "$VERSION" ] && VERSION=$(cat VERSION)
[ -z "$BUILDTIME" ] && BUILDTIME=$(TZ=GMT date "+%Y-%m-%d_%H:%M_GMT")
[ -z "$GITCOMMIT" ] && GITCOMMIT=$(git rev-parse HEAD --short 2>/dev/null)
[ -z "$GITBRANCH" ] && GITBRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

go_build() {
  [ -d "${DIST}" ] && rm -rf "${DIST}/*"
  [ -d "${DIST}" ] || mkdir -p "${DIST}"
  glide install
  CGO_ENABLED=0 go build \
    -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GITCOMMIT} -X main.GitBranch=${GITBRANCH} -X main.BuildTime=${BUILDTIME}" \
    -v -o "${DIST}/pumba"
}

go_build
