#!/bin/sh
# GoatCounter 启动脚本:首次启动时创建 SQLite 库与站点账号,之后直接 serve。
# TLS 由主机上的外层反代终结,容器内只跑 HTTP。
set -eu

DB="sqlite+/data/goatcounter.sqlite3"

: "${GC_VHOST:?GC_VHOST is required}"

if [ ! -f /data/goatcounter.sqlite3 ]; then
    : "${GC_EMAIL:?GC_EMAIL is required for first run}"
    : "${GC_PASSWORD:?GC_PASSWORD is required for first run}"
    case "${GC_PASSWORD}" in
        __REQUIRED_*|*change-me*)
            echo "GC_PASSWORD is still a placeholder" >&2
            exit 1
            ;;
    esac
    goatcounter db create site \
        -db "${DB}" -createdb \
        -vhost "${GC_VHOST}" \
        -user.email "${GC_EMAIL}" \
        -user.password "${GC_PASSWORD}"
fi

exec goatcounter serve -db "${DB}" -listen :8080 -tls http
