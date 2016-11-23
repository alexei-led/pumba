FROM alexeiled/go-builder:1.7.3-onbuild

MAINTAINER Alexei Ledenev <alexei.led@gmail.com>

RUN mkdir -p /go/src/github.com/gaia-adm && \
    cd /go/src/github.com/gaia-adm && \
    ln -s /go/src/app pumba

WORKDIR /go/src/github.com/gaia-adm/pumba

CMD ["script/go_build.sh"]
