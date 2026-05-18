#!/bin/bash

set -euo pipefail

APP_SERVICE_OVERRIDE="${APP_SERVICE-}"
FRP_SERVICE_OVERRIDE="${FRP_SERVICE-}"
START_SERVICES_OVERRIDE="${START_SERVICES-}"

log() {
    echo "[update] $*"
}

normalize_service_name() {
    local name
    name="${1:-}"
    name="${name%.service}"
    echo "$name"
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
}

main() {
    need_root "$@"

    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$script_dir"
    load_env

    log "停止 services..."
    systemctl stop "$APP_SERVICE" 2>/dev/null || true
    systemctl stop "$FRP_SERVICE" 2>/dev/null || true

    log "拉取最新代码..."
    git pull --ff-only

    log "重新编译前后端..."
    bash ./build.sh

    systemctl daemon-reload
    if [ "$START_SERVICES" = "1" ]; then
        log "启动 services..."
        systemctl start "$FRP_SERVICE" 2>/dev/null || log "${FRP_SERVICE} 启动失败，请检查 FRP 配置"
        systemctl start "$APP_SERVICE"
    else
        log "START_SERVICES=0，已跳过启动 services"
    fi

    log "更新完成"
}

main "$@"
