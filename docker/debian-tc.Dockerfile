FROM debian:stable-slim

RUN apt-get update && apt-get install iproute2 -y

ENTRYPOINT ["/sbin/tc"]