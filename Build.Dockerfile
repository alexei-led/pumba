FROM golang:1.7.3-alpine

MAINTAINER Alexei Ledenev <alexei.led@gmail.com>

# install Git apk
RUN apk --no-cache add git bash curl

# install glide package manager
ENV GLIDE_VERSION v0.12.3
RUN curl -Ls https://github.com/Masterminds/glide/releases/download/$GLIDE_VERSION/glide-$GLIDE_VERSION-linux-amd64.tar.gz | tar xz -C /tmp \
 && mv /tmp/linux-amd64/glide /usr/bin/

# gox - Go cross compile tool
# github-release - Github Release and upload artifacts
# goveralls - Go integration for Coveralls.io
# cover - Go code coverage tool
# go-junit-report - convert Go test into junit.xml format
RUN go get -v github.com/mitchellh/gox && \
    go get -v github.com/aktau/github-release && \
    go get -v github.com/mattn/goveralls && \
    go get -v golang.org/x/tools/cmd/cover && \
    go get -v github.com/jstemmer/go-junit-report

# prepare work directory
ENV PUMBADIR /go/src/github.com/gaia-adm/pumba
RUN mkdir -p $PUMBADIR
WORKDIR $PUMBADIR

# install dependencies
COPY glide.* ./
RUN glide install

# add source files
COPY . .

CMD ["script/go_build.sh"]
