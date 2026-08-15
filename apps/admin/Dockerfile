FROM node:26-alpine@sha256:aadf416b2cdce311a8811ba3f0608a61b77dbf997500e2eafe781b51f6a0b019 AS builder

WORKDIR /src

COPY apps/admin/package*.json ./
RUN npm ci --fetch-retries=3 --fetch-retry-mintimeout=10000 --fetch-retry-maxtimeout=60000

COPY apps/admin ./
RUN npm run build

FROM nginx:1.30-alpine@sha256:97d490c12ba55b4946b01546d1c3ed324e8d41ab1c9fcb2a616aa470620e5b46

COPY apps/admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY apps/admin/nginx/admin-security-headers.conf /etc/nginx/snippets/admin-security-headers.conf
COPY --from=builder /src/dist /usr/share/nginx/html
COPY apps/admin/docker-entrypoint.d/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-config.sh

EXPOSE 80
