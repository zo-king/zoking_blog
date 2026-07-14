FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/seed ./cmd/seed

FROM debian:bookworm-slim

ARG HUGO_VERSION=0.160.1
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata curl tar \
    && curl -fsSL "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.tar.gz" \
    | tar -xz -C /usr/local/bin hugo \
    && chmod +x /usr/local/bin/hugo \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid 10001 zoking \
    && useradd --uid 10001 --gid 10001 --system --no-create-home --home-dir /nonexistent zoking \
    && mkdir -p /workspace/apps/site /data/media /data/releases /data/current /data/previews \
    && chown -R 10001:10001 /workspace /data

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/worker /app/worker
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/seed /app/seed

ENV APP_ENV=production
ENV PATH="/usr/local/bin:${PATH}"
EXPOSE 18080

USER 10001:10001
CMD ["/app/api"]
