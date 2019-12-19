FROM golang:1.12-alpine

RUN apk add --no-cache git

RUN mkdir -p /go/src/github.com/alexei-led/pumba
WORKDIR /go/src/github.com/alexei-led/pumba
RUN go get github.com/vektra/mockery/.../

ENV CGO_ENABLED=0

ENTRYPOINT [ "/go/bin/mockery" ]