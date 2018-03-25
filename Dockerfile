#
# ----- Go Builder Image ------
#
FROM golang:1.10-alpine AS builder

# github-release - Github Release and upload artifacts
# go-junit-report - convert Go test into junit.xml format
RUN apk add --no-cache git bash curl || apk update && apk upgrade
RUN go get -v github.com/aktau/github-release && \
    go get -v github.com/jstemmer/go-junit-report

# set working directory
RUN mkdir -p /go/src/github.com/alexei-led/pumba
WORKDIR /go/src/github.com/alexei-led/pumba

# copy sources (including .git repo)
COPY . .

# set entrypoint to bash
ENTRYPOINT ["/bin/bash"]

# build argument to secify if to create a GitHub release
ARG RELEASE=false
ARG DEBUG=false

# run test and calculate coverage: skip for RELEASE
RUN if [[ "$RELEASE" == false ]]; then VERSION=$(cat VERSION) script/test.sh; fi

# upload coverage reports to Codecov.io: pass CODECOV_TOKEN as build-arg: skip for RELEASE
ARG CODECOV_TOKEN
RUN if [[ "$RELEASE" == false ]]; then bash -c "bash <(curl -s https://codecov.io/bash) -t ${CODECOV_TOKEN}"; fi

# build pumba binary for amd64 linux
RUN VERSION=$(cat VERSION) script/go_build.sh

# build pumba for all platforms
RUN if [[ "$RELEASE" == true ]]; then VERSION=$(cat VERSION) script/gox_build.sh; fi

# release to GitHub; pass GITHUB_TOKEN as build-arg
ARG GITHUB_TOKEN
RUN if [[ "$RELEASE" == true ]]; then RELEASE_TAG=$(git describe --abbrev=0) TAG_MESSAGE="$(git tag -l $RELEASE_TAG -n 20 | awk '{$1=""; print}')" script/github_release.sh alexei-led pumba; fi

#
# ------ Pumba runtime image ------
#
FROM alpine:3.7

LABEL com.gaiaadm.pumba=true

RUN addgroup pumba && adduser -s /bin/bash -D -G pumba pumba

RUN apk add --no-cache dumb-init su-exec

COPY --from=builder /go/src/github.com/alexei-led/pumba/dist/bin/pumba /usr/bin/pumba
COPY docker_entrypoint.sh /

ENTRYPOINT ["dumb-init", "/docker_entrypoint.sh"]
CMD ["pumba", "--help"]

ARG GH_SHA=dev
LABEL org.label-schema.vcs-ref=$GH_SHA \
      org.label-schema.vcs-url="https://github.com/alexei-led/pumba"
