FROM alexeiled/go-builder:1.7.3-onbuild

MAINTAINER Alexei Ledenev <alexei.led@gmail.com>

RUN mkdir -p /go/src/github.com/slnowak && \
    cd /go/src/github.com/slnowak && \
    ln -s /go/src/app pumba

WORKDIR /go/src/github.com/slnowak/pumba

CMD ["script/go_build.sh"]
