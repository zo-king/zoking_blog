FROM debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

ARG GOATCOUNTER_VERSION=2.7.0
ARG GOATCOUNTER_SHA256=98d221cb9c8ef2bf76d8daa9cca647839f8d8b0bb5bc7400ff9337c5da834511
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata curl gzip \
    && curl -fsSL -o /tmp/goatcounter.gz "https://github.com/arp242/goatcounter/releases/download/v${GOATCOUNTER_VERSION}/goatcounter-v${GOATCOUNTER_VERSION}-linux-amd64.gz" \
    && echo "${GOATCOUNTER_SHA256}  /tmp/goatcounter.gz" | sha256sum -c - \
    && gzip -d -c /tmp/goatcounter.gz > /usr/local/bin/goatcounter \
    && chmod +x /usr/local/bin/goatcounter \
    && rm -f /tmp/goatcounter.gz \
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
