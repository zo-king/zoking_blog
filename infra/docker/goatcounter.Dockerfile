FROM golang:1.27.0-bookworm@sha256:484ef6066fa69acb059fdfeda7ba2b8f7391f2ef6abc6f9b8411e669ebd56466 AS builder

WORKDIR /src
ARG GOPROXY=https://proxy.golang.org,direct
COPY infra/docker/goatcounter-build/go.mod infra/docker/goatcounter-build/go.sum ./
COPY infra/docker/goatcounter-healthcheck.go ./healthcheck.go
RUN GOPROXY=${GOPROXY} go mod download \
    && CGO_ENABLED=1 GOPROXY=${GOPROXY} go build -trimpath \
        -ldflags='-s -w -extldflags=-static' \
        -tags='osusergo,netgo,sqlite_omit_load_extension' \
        -o /out/goatcounter zgo.at/goatcounter/v2/cmd/goatcounter \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/goatcounter-healthcheck ./healthcheck.go

FROM debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

COPY --from=builder /out/goatcounter /usr/local/bin/goatcounter
COPY --from=builder /out/goatcounter-healthcheck /usr/local/bin/goatcounter-healthcheck
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

RUN groupadd --gid 10002 goatcounter \
    && useradd --uid 10002 --gid 10002 --system --no-create-home --home-dir /nonexistent goatcounter \
    && mkdir -p /data \
    && chown 10002:10002 /data

COPY infra/docker/goatcounter-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER 10002:10002
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
