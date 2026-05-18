#!/bin/bash

set -euo pipefail

APP_SERVICE_OVERRIDE="${APP_SERVICE-}"
FRP_SERVICE_OVERRIDE="${FRP_SERVICE-}"
START_SERVICES_OVERRIDE="${START_SERVICES-}"
GITHUB_PROXY_OVERRIDE="${GITHUB_PROXY-}"
HTTP_PROXY_OVERRIDE="${HTTP_PROXY-}"
HTTPS_PROXY_OVERRIDE="${HTTPS_PROXY-}"
ALL_PROXY_OVERRIDE="${ALL_PROXY-}"
NO_PROXY_OVERRIDE="${NO_PROXY-}"

log() {
    echo "[update] $*"
}

normalize_service_name() {
    local name
    name="${1:-}"
    name="${name%.service}"
    echo "$name"
}

normalize_url_prefix() {
    local prefix
    prefix="${1:-}"
    if [ -n "$prefix" ]; then
        prefix="${prefix%/}/"
    fi
    echo "$prefix"
}

need_root() {
    if [ "$(id -u)" -ne 0 ]; then
        exec sudo -E bash "$0" "$@"
    fi
}

load_env() {
    if [ -f .env ]; then
        set -a
        # shellcheck disable=SC1091
        . ./.env
        set +a
    fi

    APP_SERVICE="$(normalize_service_name "${APP_SERVICE_OVERRIDE:-${APP_SERVICE:-labpanel}}")"
    FRP_SERVICE="$(normalize_service_name "${FRP_SERVICE_OVERRIDE:-${FRP_SERVICE:-${SERVICE_NAME:-frpc}}}")"
    START_SERVICES="${START_SERVICES_OVERRIDE:-${START_SERVICES:-1}}"
    GITHUB_PROXY="$(normalize_url_prefix "${GITHUB_PROXY_OVERRIDE:-${GITHUB_PROXY:-}}")"
    HTTP_PROXY="${HTTP_PROXY_OVERRIDE:-${HTTP_PROXY:-${http_proxy:-}}}"
    HTTPS_PROXY="${HTTPS_PROXY_OVERRIDE:-${HTTPS_PROXY:-${https_proxy:-$HTTP_PROXY}}}"
    ALL_PROXY="${ALL_PROXY_OVERRIDE:-${ALL_PROXY:-${all_proxy:-}}}"
    NO_PROXY="${NO_PROXY_OVERRIDE:-${NO_PROXY:-${no_proxy:-}}}"
}

configure_network() {
    if [ -n "$HTTP_PROXY" ]; then
        export HTTP_PROXY="$HTTP_PROXY" http_proxy="$HTTP_PROXY"
    fi
    if [ -n "$HTTPS_PROXY" ]; then
        export HTTPS_PROXY="$HTTPS_PROXY" https_proxy="$HTTPS_PROXY"
    fi
    if [ -n "$ALL_PROXY" ]; then
        export ALL_PROXY="$ALL_PROXY" all_proxy="$ALL_PROXY"
    fi
    if [ -n "$NO_PROXY" ]; then
        export NO_PROXY="$NO_PROXY" no_proxy="$NO_PROXY"
    fi
}

git_cmd() {
    if [ -n "$GITHUB_PROXY" ]; then
        git \
            -c "url.${GITHUB_PROXY}https://github.com/.insteadOf=https://github.com/" \
            -c "url.${GITHUB_PROXY}https://github.com/.insteadOf=git@github.com:" \
            "$@"
    else
        git "$@"
    fi
}

main() {
    need_root "$@"

    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$script_dir"
    load_env
    configure_network

    log "停止 services..."
    systemctl stop "$APP_SERVICE" 2>/dev/null || true

    log "拉取最新代码..."
    git_cmd pull --ff-only

    log "重新编译前后端..."
    bash ./build.sh

    systemctl daemon-reload
    if [ "$START_SERVICES" = "1" ]; then
        log "启动 services..."
        systemctl start "$APP_SERVICE"
    else
        log "START_SERVICES=0，已跳过启动 services"
    fi

    log "更新完成"
}

main "$@"
