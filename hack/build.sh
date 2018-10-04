#!/bin/bash
[ -z "$DIST" ] && DIST=.bin

[ -z "$VERSION" ] && VERSION=$(cat VERSION)
[ -z "$BUILDTIME" ] && BUILDTIME=$(TZ=GMT date "+%Y-%m-%d_%H:%M_GMT")
[ -z "$VCS_COMMIT_ID" ] && VCS_COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null)
[ -z "$VCS_BRANCH_NAME" ] && VCS_BRANCH_NAME=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

go_build() {
  [ -d "${DIST}" ] && rm -rf "${DIST:?}/*"
  [ -d "${DIST}" ] || mkdir -p "${DIST}"
  CGO_ENABLED=0 go build \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.GitCommit=${VCS_COMMIT_ID} -X main.GitBranch=${VCS_BRANCH_NAME} -X main.BuildTime=${BUILDTIME}" \
    -v -o "${DIST}/pumba" ./cmd
}

go_build
