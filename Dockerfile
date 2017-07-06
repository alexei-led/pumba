#
# ----- Go Builder Image ------
#
FROM golang:1.8-alpine AS builder

# gox - Go cross compile tool
# github-release - Github Release and upload artifacts
# go-junit-report - convert Go test into junit.xml format
RUN apk add --no-cache git bash curl && \
    go get -v github.com/mitchellh/gox && \
    go get -v github.com/aktau/github-release && \
    go get -v github.com/jstemmer/go-junit-report

# set working directory
RUN mkdir -p /go/src/github.com/gaia-adm/pumba
WORKDIR /go/src/github.com/gaia-adm/pumba

# copy sources (including .git repo)
COPY . .

# set entrypoint to bash
ENTRYPOINT ["/bin/bash"]

# build argument to secify if to create a GitHub release
ARG RELEASE=false
ARG DEBUG=false

# run test and calculate coverage: skip for RELEASE
RUN if [[ "$RELEASE" == false ]]; then VERSION=$(cat VERSION) script/coverage.sh; fi

# upload coverage reports to Codecov.io: pass CODECOV_TOKEN as build-arg: skip for RELEASE
ARG CODECOV_TOKEN
RUN if [[ "$RELEASE" == false ]]; then bash -c "bash <(curl -s https://codecov.io/bash) -t ${CODECOV_TOKEN}"; fi

# build pumba binary for amd64 linux
RUN VERSION=$(cat VERSION) script/go_build.sh

# build pumba for all platforms
RUN if [[ "$RELEASE" == true ]]; then VERSION=$(cat VERSION) script/gox_build.sh; fi

# release to GitHub; pass GITHUB_TOKEN as build-arg
ARG GITHUB_TOKEN
RUN if [[ "$RELEASE" == true ]]; then RELEASE_TAG=$(git describe --abbrev=0) TAG_MESSAGE=$(git tag -l $RELEASE_TAG -n 20 | awk '{$1=""; print}') script/github_release.sh gaia-adm pumba; fi

#
# ------ Pumba runtime image ------
#
FROM alpine:3.6

LABEL com.gaiaadm.pumba=true

RUN addgroup pumba && adduser -s /bin/bash -D -G pumba pumba

ENV GOSU_VERSION 1.10
ENV GOSU_SHA_256 5b3b03713a888cee84ecbf4582b21ac9fd46c3d935ff2d7ea25dd5055d302d3c

RUN apk add --no-cache --virtual .gosu-deps curl && \
    curl -o /tmp/gosu-amd64 -LS  "https://github.com/tianon/gosu/releases/download/$GOSU_VERSION/gosu-amd64" && \
    echo "${GOSU_SHA_256}  gosu-amd64" > /tmp/gosu-amd64.sha256 && \
    cd /tmp && sha256sum -c gosu-amd64.sha256 && \
    mv /tmp/gosu-amd64 /usr/local/bin/gosu && \
    chmod +x /usr/local/bin/gosu && \
    gosu nobody true && \
    rm -rf /tmp/* && \
    apk del .gosu-deps

COPY --from=builder /go/src/github.com/gaia-adm/pumba/dist/bin/pumba /usr/bin/pumba
COPY docker_entrypoint.sh /
RUN chmod +x /docker_entrypoint.sh

ENTRYPOINT ["/docker_entrypoint.sh"]
CMD ["pumba", "--help"]

ARG GH_SHA=dev
LABEL org.label-schema.vcs-ref=$GH_SHA \
      org.label-schema.vcs-url="https://github.com/gaia-adm/pumba"
