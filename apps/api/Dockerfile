FROM golang:1.26.5-bookworm@sha256:6c5605ab3a9a9fb3c4eafe5b3d63cdbf3881caf113262b67862547b54a9db599 AS builder

WORKDIR /src

ARG GOPROXY=https://proxy.golang.org,direct

COPY apps/api/go.mod apps/api/go.sum ./
RUN GOPROXY=${GOPROXY} go mod download

COPY apps/api ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/seed ./cmd/seed

FROM debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

ARG HUGO_VERSION=0.160.1
ARG HUGO_SHA256=133adf4932c1b626b6fe6aa28d56791555abe3ceff167e03be534e8324c9ed39
ARG PAGEFIND_VERSION=1.5.2
ARG PAGEFIND_SHA256=aeb135927856e7a49816cf2c9c2e53167f87afa1f61f7288b7d66c234a950d38
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata curl tar \
    && curl -fsSL -o /tmp/hugo.tar.gz "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-amd64.tar.gz" \
    && echo "${HUGO_SHA256}  /tmp/hugo.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/hugo.tar.gz -C /usr/local/bin hugo \
    && chmod +x /usr/local/bin/hugo \
    && curl -fsSL -o /tmp/pagefind.tar.gz "https://github.com/Pagefind/pagefind/releases/download/v${PAGEFIND_VERSION}/pagefind_extended-v${PAGEFIND_VERSION}-x86_64-unknown-linux-musl.tar.gz" \
    && echo "${PAGEFIND_SHA256}  /tmp/pagefind.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/pagefind.tar.gz -C /usr/local/bin \
    && mv /usr/local/bin/pagefind_extended /usr/local/bin/pagefind \
    && chmod +x /usr/local/bin/pagefind \
    && rm -f /tmp/hugo.tar.gz /tmp/pagefind.tar.gz \
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
COPY --from=builder /usr/local/go /usr/local/go

# The site module replaces the upstream theme with the repository root.
# Production publishing therefore needs both the Go module runner and theme files.
COPY go.mod theme.toml /workspace/
COPY archetypes /workspace/archetypes
COPY assets /workspace/assets
COPY config /workspace/config
COPY data /workspace/data
COPY i18n /workspace/i18n
COPY layouts /workspace/layouts

ENV APP_ENV=production
ENV PATH="/usr/local/go/bin:/usr/local/bin:${PATH}"
EXPOSE 18080

USER 10001:10001
CMD ["/app/api"]
