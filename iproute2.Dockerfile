FROM alpine:3.3

MAINTAINER Alexei Ledenev <alexei.led@gmail.com>

LABEL com.gaiaadm.pumba.skip=true

RUN apk --no-cache add iproute2

ENTRYPOINT ["tc"]
