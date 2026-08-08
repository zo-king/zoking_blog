FROM golang:1.26.5-bookworm@sha256:6c5605ab3a9a9fb3c4eafe5b3d63cdbf3881caf113262b67862547b54a9db599 AS builder

WORKDIR /src
ARG GOPROXY=https://proxy.golang.org,direct
COPY infra/docker/goatcounter-build/go.mod infra/docker/goatcounter-build/go.sum ./
RUN GOPROXY=${GOPROXY} go mod download \
    && CGO_ENABLED=1 GOPROXY=${GOPROXY} go build -trimpath \
        -ldflags='-s -w -extldflags=-static' \
        -tags='osusergo,netgo,sqlite_omit_load_extension' \
        -o /out/goatcounter zgo.at/goatcounter/v2/cmd/goatcounter

FROM debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

RUN sed -i "s|http://deb.debian.org|https://deb.debian.org|g" /etc/apt/sources.list.d/debian.sources \
    && apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/goatcounter /usr/local/bin/goatcounter

RUN groupadd --gid 10002 goatcounter \
    && useradd --uid 10002 --gid 10002 --system --no-create-home --home-dir /nonexistent goatcounter \
    && mkdir -p /data \
    && chown 10002:10002 /data

COPY infra/docker/goatcounter-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER 10002:10002
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
