#!/bin/bash
[ -z "$DIST" ] && DIST=dist/bin

[ -z "$VERSION" ] && VERSION=$(cat VERSION)
[ -z "$BUILDTIME" ] && BUILDTIME=$(TZ=GMT date "+%Y-%m-%d_%H:%M_GMT")
[ -z "$GITCOMMIT" ] && GITCOMMIT=$(git rev-parse HEAD --short 2>/dev/null)
[ -z "$GITBRANCH" ] && GITBRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

[ -d "${DIST}" ] && rm -rf "${DIST:?}/*"
[ -d "${DIST}" ] || mkdir -p "${DIST}"
echo "Building ${BUILD_VERSION} on ${BUILD_DATE}"

platforms=("windows/amd64" "linux/amd64" "darwin/amd64" "linux/386")

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output_name="pumba_${GOOS}_${GOARCH}"
    if [ $GOOS = "windows" ]; then
        output_name+='.exe'
    fi  

    echo "Building pumba for ${GOOS}/${GOARCH}..."
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
    -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GITCOMMIT} -X main.GitBranch=${GITBRANCH} -X main.BuildTime=${BUILDTIME}" \
    -o "${DIST}/${output_name}"
    if [ $? -ne 0 ]; then
        echo 'An error has occurred! Aborting the script execution...'
        exit 1
    fi
done
