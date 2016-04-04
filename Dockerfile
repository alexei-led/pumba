FROM alpine:3.3

MAINTAINER Alexei Ledenev <alexei.led@gmail.com>

COPY .dist/pumba /usr/bin/pumba

CMD ["--help"]
ENTRYPOINT ["/usr/bin/pumba"]
