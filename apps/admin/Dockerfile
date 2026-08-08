FROM node:25-alpine@sha256:bdf2cca6fe3dabd014ea60163eca3f0f7015fbd5c7ee1b0e9ccb4ced6eb02ef4 AS builder

WORKDIR /src

COPY apps/admin/package*.json ./
RUN npm ci --fetch-retries=3 --fetch-retry-mintimeout=10000 --fetch-retry-maxtimeout=60000

COPY apps/admin ./
RUN npm run build

FROM nginx:1.27-alpine@sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10

COPY apps/admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY apps/admin/nginx/admin-security-headers.conf /etc/nginx/snippets/admin-security-headers.conf
COPY --from=builder /src/dist /usr/share/nginx/html
COPY apps/admin/docker-entrypoint.d/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-config.sh

EXPOSE 80
