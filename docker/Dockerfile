#
# ----- Go Builder Image ------
#
FROM golang:1.19 AS builder

# curl git bash
RUN apt-get update && apt-get install -y --no-install-recommends \
        curl \
        git \
        bash \
        tzdata \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

#
# ----- Build and Test Image -----
#
FROM builder as build-and-test

# set working directory
RUN mkdir -p /go/src/pumba
WORKDIR /go/src/pumba

# copy Makefile
COPY Makefile .
RUN --mount=type=cache,target=/root/.cache/go-build make setup-tools

# copy go.mod/sum
COPY go.* ./
RUN --mount=type=cache,target=/root/.cache/go-build make dependency

# copy sources
COPY mocks ./mocks
COPY cmd ./cmd
COPY pkg ./pkg
COPY .golangci.yml .
COPY VERSION .

# run lint, test race and calculate coverage
ARG SKIP_TESTS
RUN --mount=type=cache,target=/root/.cache/go-build if [ -z "$SKIP_TESTS" -o "$SKIP_TESTS" = false ]; then make lint test-race test-coverage; fi

# build pumba binary for TARGETOS/TARGETARCH (default: linux/amd64)
# passed by buildkit
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build TARGETOS=${TARGETOS} TARGETARCH=${TARGETARCH} make build


#
# ------ Pumba Integration Tests ------
#
FROM bats/bats:1.8.2 as integration-tests

# install required packages
RUN apk add --no-cache docker iproute2

# copy bats tests
COPY ./tests /code/tests
COPY VERSION .

# copy compiled binary
COPY --from=build-and-test /go/src/pumba/.bin/github.com/alexei-led/pumba /usr/local/bin/pumba

# mount docker.sock and run pumba tests
ENTRYPOINT [ "bash", "-c" ]
CMD [ "[ -e /var/run/docker.sock ] && bats --print-output-on-failure /tests" ]

#
# ------ Pumba release Docker image ------
#
FROM scratch

# copy certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# copy timezone settings
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# this is the last command since it's never cached
COPY --from=build-and-test /go/src/pumba/.bin/github.com/alexei-led/pumba /pumba

ENTRYPOINT ["/pumba"]