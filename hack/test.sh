#!/bin/bash
# Generate test coverage statistics for Go packages.
#
# Works around the fact that `go test -coverprofile` currently does not work
# with multiple packages, see https://code.google.com/p/go/issues/detail?id=6909
#
# Usage: hack/test
#
#     --html  Additionally create HTML report and open it in browser
#
set -e
set -o pipefail

[ -z "$COVER" ] && COVER=.cover
profile="$COVER/cover.out"
mode=atomic

OS=$(uname)
race_flag="-race"
cgo_flag=1
if [ "$OS" = "Linux" ]; then
  # check Alpine - alpine does not support race test
  if [ -f "/etc/alpine-release" ]; then 
    race_flag=""
    cgo_flag=0
  fi
fi
if [ "$race_flag" != "" ]; then
  echo "testing with race detection ..."
fi

generate_cover_data() {
  [ -d "${COVER}" ] && rm -rf "${COVER:?}/*"
  [ -d "${COVER}" ] || mkdir -p "${COVER}"

  # Save current IFS
  SAVEIFS=$IFS
  # Change IFS to new line. 
  IFS=$'\n'
  pkgs=($(go list -f '{{if .TestGoFiles}}{{ .ImportPath }}{{end}}' ./... | grep -v vendor))
  # Restore IFS
  # Restore IFS
  IFS=$SAVEIFS

  for pkg in "${pkgs[@]}"; do
    f="${COVER}/$(echo $pkg | tr / -).cover"
    tout="${COVER}/$(echo $pkg | tr / -)_tests.out"
    CGO_ENABLED=$cgo_flag go test -v $race_flag -covermode="$mode" -coverprofile="$f" "$pkg" | tee "$tout"
  done

  echo "mode: $mode" >"$profile"
  grep -h -v "^mode:" "${COVER}"/*.cover >>"$profile"
}

generate_cover_report() {
  go tool cover -${1}="$profile" -o "${COVER}/coverage.html"
}

generate_cover_data
generate_cover_report html
