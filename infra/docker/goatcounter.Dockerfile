FROM debian:bookworm-slim

ARG GOATCOUNTER_VERSION=2.7.0
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata curl gzip \
    && curl -fsSL "https://github.com/arp242/goatcounter/releases/download/v${GOATCOUNTER_VERSION}/goatcounter-v${GOATCOUNTER_VERSION}-linux-amd64.gz" \
    | gzip -d > /usr/local/bin/goatcounter \
    && chmod +x /usr/local/bin/goatcounter \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid 10002 goatcounter \
    && useradd --uid 10002 --gid 10002 --system --no-create-home --home-dir /nonexistent goatcounter \
    && mkdir -p /data \
    && chown 10002:10002 /data

COPY infra/docker/goatcounter-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER 10002:10002
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
