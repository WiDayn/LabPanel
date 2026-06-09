#!/bin/bash

set -euo pipefail

APP_SERVICE_OVERRIDE="${APP_SERVICE-}"
FRP_SERVICE_OVERRIDE="${FRP_SERVICE-}"
START_SERVICES_OVERRIDE="${START_SERVICES-}"
GITHUB_PROXY_OVERRIDE="${GITHUB_PROXY-}"
DEFAULT_GITHUB_PROXY_OVERRIDE="${DEFAULT_GITHUB_PROXY-}"
HTTP_PROXY_OVERRIDE="${HTTP_PROXY-}"
HTTPS_PROXY_OVERRIDE="${HTTPS_PROXY-}"
ALL_PROXY_OVERRIDE="${ALL_PROXY-}"
NO_PROXY_OVERRIDE="${NO_PROXY-}"
GIT_TRANSPORT_OVERRIDE="${GIT_TRANSPORT-}"
GITHUB_ROUTE_OVERRIDE="${GITHUB_ROUTE-}"
RUN_USER=""
SELECTED_GITHUB_PROXY=""
TMP_BINARY=""
TMP_DIST=""
BACKUP_BINARY=""
BACKUP_DIST=""

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

is_disabled_value() {
    case "${1:-}" in
        0|off|OFF|no|NO|none|NONE|false|FALSE) return 0 ;;
        *) return 1 ;;
    esac
}

github_probe_url() {
    local proxy_prefix url
    proxy_prefix="$1"
    url="https://github.com/WiDayn/LabPanel.git"
    if [ -n "$proxy_prefix" ]; then
        echo "${proxy_prefix}${url}"
    else
        echo "$url"
    fi
}

now_ms() {
    local now
    now="$(date +%s%3N 2>/dev/null || true)"
    case "$now" in
        *N*|"") echo "$(date +%s)000" ;;
        *) echo "$now" ;;
    esac
}

measure_github_route() {
    local proxy_prefix url start end
    proxy_prefix="$1"
    url="$(github_probe_url "$proxy_prefix")"
    start="$(now_ms)"
    if command -v timeout >/dev/null 2>&1; then
        GIT_TERMINAL_PROMPT=0 timeout 10 git -c http.lowSpeedLimit=1 -c http.lowSpeedTime=8 ls-remote --heads "$url" main >/dev/null 2>&1 || return 0
    else
        GIT_TERMINAL_PROMPT=0 git -c http.lowSpeedLimit=1 -c http.lowSpeedTime=8 ls-remote --heads "$url" main >/dev/null 2>&1 || return 0
    fi
    end="$(now_ms)"
    awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000}"
}

select_github_route() {
    local proxy_candidate direct_time proxy_time chosen
    SELECTED_GITHUB_PROXY=""

    if [ "$GIT_TRANSPORT" != "https" ]; then
        log "GitHub 线路: GIT_TRANSPORT=${GIT_TRANSPORT}，跳过 HTTPS 线路测速"
        return
    fi

    if is_disabled_value "$GITHUB_PROXY"; then
        log "GitHub 线路: 已禁用代理，使用直连"
        return
    fi

    proxy_candidate="$(normalize_url_prefix "${GITHUB_PROXY:-$DEFAULT_GITHUB_PROXY}")"
    [ -n "$proxy_candidate" ] || return

    case "$GITHUB_ROUTE" in
        github)
            log "GitHub 线路: 使用 GitHub 直连"
            return
            ;;
        gh-proxy)
            SELECTED_GITHUB_PROXY="$proxy_candidate"
            log "GitHub 线路: 使用代理 ${proxy_candidate}"
            return
            ;;
        auto|"")
            ;;
        *)
            log "GitHub 线路: 未知 GITHUB_ROUTE=${GITHUB_ROUTE}，使用自动测速"
            ;;
    esac

    if ! command -v git >/dev/null 2>&1; then
        SELECTED_GITHUB_PROXY="$proxy_candidate"
        log "git 不可用，暂用 GitHub 代理候选: ${proxy_candidate}"
        return
    fi

    direct_time="$(measure_github_route "")"
    proxy_time="$(measure_github_route "$proxy_candidate")"

    if [ -n "$direct_time" ] && [ -n "$proxy_time" ]; then
        if awk "BEGIN {exit !($proxy_time < $direct_time)}"; then
            SELECTED_GITHUB_PROXY="$proxy_candidate"
            chosen="代理"
        else
            chosen="直连"
        fi
        log "GitHub 线路测速: 直连 ${direct_time}s，代理 ${proxy_time}s，使用${chosen}"
    elif [ -n "$proxy_time" ]; then
        SELECTED_GITHUB_PROXY="$proxy_candidate"
        log "GitHub 直连不可用，使用代理 ${proxy_candidate}"
    elif [ -n "$direct_time" ]; then
        log "GitHub 代理不可用，使用直连"
    else
        log "GitHub 线路测速失败，默认使用直连"
    fi
}

run_sudo() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    else
        sudo -E "$@"
    fi
}

detect_run_user() {
    local script_dir owner
    script_dir="${1:-}"
    if [ "$(id -u)" -ne 0 ]; then
        RUN_USER=""
        return
    fi

    if [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
        RUN_USER="$SUDO_USER"
        return
    fi

    owner="$(stat -c '%U' "$script_dir" 2>/dev/null || true)"
    if [ -n "$owner" ] && [ "$owner" != "root" ]; then
        RUN_USER="$owner"
    fi
}

run_user() {
    if [ -n "$RUN_USER" ]; then
        sudo -E -H -u "$RUN_USER" "$@"
    else
        "$@"
    fi
}

load_env() {
    if [ -f .env ]; then
        set -a
        # shellcheck disable=SC1091
        . ./.env
        set +a
    fi

    APP_SERVICE="$(normalize_service_name "${APP_SERVICE_OVERRIDE:-${APP_SERVICE:-lab-panel}}")"
    FRP_SERVICE="$(normalize_service_name "${FRP_SERVICE_OVERRIDE:-${FRP_SERVICE:-${SERVICE_NAME:-frpc}}}")"
    START_SERVICES="${START_SERVICES_OVERRIDE:-${START_SERVICES:-1}}"
    GITHUB_PROXY="${GITHUB_PROXY_OVERRIDE:-${GITHUB_PROXY:-}}"
    if ! is_disabled_value "$GITHUB_PROXY"; then
        GITHUB_PROXY="$(normalize_url_prefix "$GITHUB_PROXY")"
    fi
    DEFAULT_GITHUB_PROXY="$(normalize_url_prefix "${DEFAULT_GITHUB_PROXY_OVERRIDE:-${DEFAULT_GITHUB_PROXY:-https://gh-proxy.com/}}")"
    HTTP_PROXY="${HTTP_PROXY_OVERRIDE:-${HTTP_PROXY:-${http_proxy:-}}}"
    HTTPS_PROXY="${HTTPS_PROXY_OVERRIDE:-${HTTPS_PROXY:-${https_proxy:-$HTTP_PROXY}}}"
    ALL_PROXY="${ALL_PROXY_OVERRIDE:-${ALL_PROXY:-${all_proxy:-}}}"
    NO_PROXY="${NO_PROXY_OVERRIDE:-${NO_PROXY:-${no_proxy:-}}}"
    GIT_TRANSPORT="${GIT_TRANSPORT_OVERRIDE:-${GIT_TRANSPORT:-https}}"
    GITHUB_ROUTE="${GITHUB_ROUTE_OVERRIDE:-${GITHUB_ROUTE:-auto}}"
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
    if [ -n "$SELECTED_GITHUB_PROXY" ]; then
        run_user git \
            -c "url.${SELECTED_GITHUB_PROXY}https://github.com/.insteadOf=https://github.com/" \
            -c "url.${SELECTED_GITHUB_PROXY}https://github.com/.insteadOf=git@github.com:" \
            "$@"
    elif [ "$GIT_TRANSPORT" = "https" ]; then
        run_user git \
            -c "url.https://github.com/.insteadOf=git@github.com:" \
            "$@"
    else
        run_user git "$@"
    fi
}


service_exists() {
    run_sudo systemctl cat "$1" >/dev/null 2>&1
}

cleanup_temp_artifacts() {
    [ -n "${TMP_BINARY:-}" ] && rm -f -- "$TMP_BINARY"
    [ -n "${TMP_DIST:-}" ] && rm -rf -- "$TMP_DIST"
}

cleanup_backup_artifacts() {
    [ -n "${BACKUP_BINARY:-}" ] && rm -f -- "$BACKUP_BINARY"
    [ -n "${BACKUP_DIST:-}" ] && rm -rf -- "$BACKUP_DIST"
}

restore_previous_artifacts() {
    log "恢复上一版构建产物..."
    [ -f LabPanel ] && rm -f -- LabPanel
    [ -d frontend/dist ] && rm -rf -- frontend/dist
    [ -n "${BACKUP_BINARY:-}" ] && [ -f "$BACKUP_BINARY" ] && mv -- "$BACKUP_BINARY" LabPanel
    [ -n "${BACKUP_DIST:-}" ] && [ -d "$BACKUP_DIST" ] && mv -- "$BACKUP_DIST" frontend/dist
}

install_built_artifacts() {
    if [ ! -f "$TMP_BINARY" ]; then
        log "未找到后端临时构建产物: $TMP_BINARY"
        return 1
    fi
    if [ ! -d "$TMP_DIST" ]; then
        log "未找到前端临时构建产物: $TMP_DIST"
        return 1
    fi

    rm -f -- "$BACKUP_BINARY"
    rm -rf -- "$BACKUP_DIST"
    [ -f LabPanel ] && mv -- LabPanel "$BACKUP_BINARY"
    [ -d frontend/dist ] && mv -- frontend/dist "$BACKUP_DIST"
    mv -- "$TMP_BINARY" LabPanel
    mv -- "$TMP_DIST" frontend/dist
}

restart_app_service() {
    if [ "$START_SERVICES" != "1" ]; then
        log "START_SERVICES=${START_SERVICES}，已跳过重启 services"
        return 0
    fi

    if ! service_exists "$APP_SERVICE"; then
        log "服务 ${APP_SERVICE}.service 不存在，请设置 APP_SERVICE 或先运行 install.sh"
        return 1
    fi

    log "重启 ${APP_SERVICE}.service..."
    if ! run_sudo systemctl restart "$APP_SERVICE"; then
        log "服务重启失败，开始回滚"
        restore_previous_artifacts
        run_sudo systemctl start "$APP_SERVICE" 2>/dev/null || true
        return 1
    fi

    if ! run_sudo systemctl is-active --quiet "$APP_SERVICE"; then
        log "服务重启后未保持 active，开始回滚"
        restore_previous_artifacts
        run_sudo systemctl start "$APP_SERVICE" 2>/dev/null || true
        return 1
    fi
}

main() {
    local script_dir suffix
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$script_dir"
    detect_run_user "$script_dir"
    load_env
    configure_network
    select_github_route

    if [ "$START_SERVICES" = "1" ] && ! service_exists "$APP_SERVICE"; then
        log "服务 ${APP_SERVICE}.service 不存在，请设置 APP_SERVICE 或先运行 install.sh"
        exit 1
    fi

    log "拉取最新代码..."
    git_cmd pull --ff-only

    suffix="$$"
    TMP_BINARY=".LabPanel.update.${suffix}"
    TMP_DIST="frontend/.dist-update.${suffix}"
    BACKUP_BINARY=".LabPanel.backup.${suffix}"
    BACKUP_DIST="frontend/.dist-backup.${suffix}"
    trap cleanup_temp_artifacts EXIT
    cleanup_temp_artifacts
    cleanup_backup_artifacts

    log "重新编译前后端..."
    run_user env OUTPUT_BINARY="$TMP_BINARY" FRONTEND_OUT_DIR=".dist-update.${suffix}" bash ./build.sh

    log "替换构建产物..."
    install_built_artifacts

    run_sudo systemctl daemon-reload
    if ! restart_app_service; then
        exit 1
    fi

    cleanup_backup_artifacts
    trap - EXIT
    cleanup_temp_artifacts

    log "更新完成"
}

main "$@"
