#
# ----- Build cg-inject binary -----
#
FROM --platform=$BUILDPLATFORM golang:1.26 AS cg-inject-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/cg-inject/ ./cmd/cg-inject/

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags='-s -w' -o /cg-inject ./cmd/cg-inject/

#
# ----- Build stress-ng from source -----
#
FROM --platform=$TARGETPLATFORM alpine:3.21 AS stress-builder

RUN apk add --no-cache build-base linux-headers git
ARG STRESS_NG_VERSION=V0.18.08
RUN git clone --depth 1 --branch ${STRESS_NG_VERSION} https://github.com/ColinIanKing/stress-ng.git /stress-ng
WORKDIR /stress-ng
RUN make -j$(nproc) STATIC=1 && strip stress-ng

#
# ----- Final scratch image -----
#
FROM scratch

LABEL com.gaiaadm.pumba.skip=true
LABEL org.opencontainers.image.source="https://github.com/alexei-led/pumba"
LABEL org.opencontainers.image.description="Minimal image with cg-inject and stress-ng for Pumba stress chaos testing"

COPY --from=cg-inject-builder /cg-inject /cg-inject
COPY --from=stress-builder /stress-ng/stress-ng /stress-ng

ENTRYPOINT ["/cg-inject"]
