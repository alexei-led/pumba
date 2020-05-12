FROM alpine:3.11

LABEL com.gaiaadm.pumba.skip=true

RUN apk --no-cache add iproute2

RUN ln -s /usr/lib/tc /lib/tc

ENTRYPOINT ["tc"]
