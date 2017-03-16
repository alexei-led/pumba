#!/bin/sh
# Generate test coverage statistics for Go packages.
#
# Works around the fact that `go test -coverprofile` currently does not work
# with multiple packages, see https://code.google.com/p/go/issues/detail?id=6909
#
# Usage: script/coverage [--html]
#
#     --html  Additionally create HTML report and open it in browser
#
[ -z "$COVER" ] && COVER=.cover
profile="$COVER/cover.out"
mode=count

generate_cover_data() {
  [ -d "${COVER}" ] && rm -rf "${COVER}/*"
  [ -d "${COVER}" ] || mkdir -p "${COVER}"

  for pkg in "$@"; do
    f="${COVER}/$(echo $pkg | tr / -).cover"
    tf="${COVER}/$(echo $pkg | tr / -)_tests.xml"
    tout="${COVER}/$(echo $pkg | tr / -)_tests.out"
    #go test -v -covermode="$mode" -coverprofile="$f" "$pkg" | go-junit-report > "$tf"
    go test -v -covermode="$mode" -coverprofile="$f" "$pkg" | tee "$tout"
    cat "$tout" | go-junit-report > "$tf"
  done

  echo "mode: $mode" >"$profile"
  grep -h -v "^mode:" "${COVER}"/*.cover >>"$profile"
}

show_cover_report() {
  go tool cover -${1}="$profile" -o "${COVER}/coverage.html"
}

push_to_coveralls() {
  if [ -z "$COVERALLS_TOKEN" ]; then
    echo "WARNING: Need to set COVERALLS_TOKEN environment variable"; exit 0
  fi
  echo "Pushing coverage statistics to coveralls.io"
  goveralls -coverprofile="$profile" -service=circle-ci -repotoken="$COVERALLS_TOKEN"
}

generate_cover_data $(go list ./... | grep -v vendor)

# if [ -z "$COVERALLS_TOKEN" ]; then
#   show_cover_report html
# else
#   push_to_coveralls
# fi
