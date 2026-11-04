FROM debian:stable-slim AS assets
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

FROM scratch
COPY --from=assets /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=assets /usr/share/zoneinfo /usr/share/zoneinfo
COPY --chmod=755 pumba /pumba
ENTRYPOINT ["/pumba"]
