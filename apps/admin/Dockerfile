FROM node:22-alpine AS builder

WORKDIR /src

COPY apps/admin/package*.json ./
RUN npm ci

COPY apps/admin ./
RUN npm run build

FROM nginx:1.27-alpine

COPY apps/admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/dist /usr/share/nginx/html
COPY apps/admin/docker-entrypoint.d/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh
RUN chmod +x /docker-entrypoint.d/40-runtime-config.sh

EXPOSE 80
