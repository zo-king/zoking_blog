FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS builder

WORKDIR /src

COPY apps/admin/package*.json ./
RUN npm ci --fetch-retries=3 --fetch-retry-mintimeout=10000 --fetch-retry-maxtimeout=60000

COPY apps/admin ./
RUN npm run build

FROM nginx:1.30-alpine@sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b

COPY apps/admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY apps/admin/nginx/admin-security-headers.conf /etc/nginx/snippets/admin-security-headers.conf
COPY --from=builder /src/dist /usr/share/nginx/html
COPY apps/admin/docker-entrypoint.d/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-config.sh

EXPOSE 80
