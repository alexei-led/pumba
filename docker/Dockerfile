#
# ----- Go Builder Image ------
#
FROM golang:1.12 AS builder

# curl git bash
RUN apt-get update && apt-get install -y --no-install-recommends \
		curl \
		git \
		bash \
	&& rm -rf /var/lib/apt/lists/*

# github-release - Github Release and upload artifacts
# go-junit-report - convert Go test into junit.xml format
RUN go get -v github.com/aktau/github-release && \
    go get -v github.com/jstemmer/go-junit-report

#
# ----- Build and Test Image -----
#
FROM builder as build-and-test

# set working directory
RUN mkdir -p /go/src/github.com/alexei-led/pumba
WORKDIR /go/src/github.com/alexei-led/pumba

# copy sources
COPY . .

# run test and calculate coverage
RUN VERSION=$(cat VERSION) hack/test.sh

# `VCS_COMMIT_ID=$(git rev-parse --short HEAD 2>/dev/null)`
ARG VCS_COMMIT_ID
ENV VCS_COMMIT_ID ${VCS_COMMIT_ID}
# `VCS_BRANCH_NAME=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)`
ARG VCS_BRANCH_NAME
ENV VCS_BRANCH_NAME ${VCS_BRANCH_NAME}
# VCS_SLUG: owner/repo slug
ARG VCS_SLUG
ENV VCS_SLUG ${VCS_SLUG}
# bild pumba binary for amd64 linux
RUN VERSION=$(cat VERSION) hack/build.sh

# upload coverage reports to Codecov.io, if CODECOV_TOKEN set through build-arg
ARG CODECOV_TOKEN
ENV CODECOV_TOKEN ${CODECOV_TOKEN}
ADD https://codecov.io/bash codecov.sh
RUN chmod +x codecov.sh

# command to upload coverage report to Codecov: need to pass CI_BUILD_ID/URL as environment variables
CMD ["./codecov.sh", "-e", "VCS_COMMIT_ID,VCS_BRANCH_NAME,VCS_SLUG,CI_BUILD_ID,CI_BUILD_URL"]

#
# ------ Pumba GitHub Release ------
#
FROM build-and-test as github-release

# build argument to secify if to create a GitHub release
ARG DEBUG=false
ARG RELEASE=false

# Release Tag: `RELEASE_TAG=$(git describe --abbrev=0)`
ARG RELEASE_TAG

# Release Tag Message: `TAG_MESSAGE=$(git tag -l $RELEASE_TAG -n 20 | awk '{$1=""; print}')`
ARG TAG_MESSAGE

# release to GitHub; pass GITHUB_TOKEN as build-arg
ARG GITHUB_TOKEN

# build pumba for all platforms
RUN if [ $RELEASE = true ]; then VERSION=$(cat VERSION) hack/xbuild.sh; fi

# release to GitHub
RUN if [ $RELEASE = true ]; then hack/github_release.sh alexei-led pumba; fi

# get latest CA certificates
FROM alpine:3.9 as certs
RUN apk --update add ca-certificates

#
# ------ Pumba runtime image ------
#
FROM scratch

# copy CA certificates
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# this is the last commabd since it's never cached
COPY --from=build-and-test /go/src/github.com/alexei-led/pumba/.bin/pumba /pumba

ENTRYPOINT ["/pumba"]