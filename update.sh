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
STAGE=""
STAGED_CHECKOUT=""
SERVICE_STOPPED=0
ARTIFACTS_SWITCHING=0
COMMITTED=0
PRESERVE_STAGE=0
SERVICE_WAS_ACTIVE=0

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
        sudo -n -E "$@"
    fi
}

app_systemctl() {
    if [ "${APP_SYSTEMD_SCOPE:-system}" = user ]; then
        systemctl --user "$@"
    else
        run_sudo systemctl "$@"
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

fetch_target() {
    local remote="$1" branch_ref="$2" remote_url github_path
    remote_url="$(run_user git remote get-url "$remote")"
    if [[ "$GIT_TRANSPORT" == https && "$remote_url" =~ (https://github\.com/|git@github\.com:|ssh://git@github\.com/)([^[:space:]]+)$ ]]; then
        github_path="${BASH_REMATCH[2]}"
        remote_url="${SELECTED_GITHUB_PROXY}https://github.com/${github_path}"
    fi
    run_user env GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND='ssh -o BatchMode=yes -o ConnectTimeout=8' \
        timeout 90 git -c http.lowSpeedLimit=1 -c http.lowSpeedTime=15 fetch --no-tags -- "$remote_url" "$branch_ref"
}

wait_for_version() {
    local expected="$1" port="$2" response deadline
    local pattern='"commit"[[:space:]]*:[[:space:]]*"'"${expected}"'"'
    deadline=$((SECONDS + 30))
    while [ "$SECONDS" -lt "$deadline" ]; do
        if app_systemctl is-active --quiet "$APP_SERVICE" &&
            response="$(curl --noproxy '*' -fsS --max-time 2 "http://127.0.0.1:${port}/api/version" 2>/dev/null)" &&
            [[ "$response" =~ $pattern ]] &&
            curl --noproxy '*' -fsS --max-time 2 -o /dev/null "http://127.0.0.1:${port}/"; then
            return 0
        fi
        sleep 1
    done
    return 1
}

finish_update() {
    local result=$?
    trap - EXIT
    trap '' INT TERM HUP
    set +e
    if [ "$COMMITTED" = 0 ] && [ "$ARTIFACTS_SWITCHING" = 1 ]; then
        log "更新失败，恢复上一版构建产物"
        if [ "$START_SERVICES" = 1 ] && ! app_systemctl stop "$APP_SERVICE"; then
            PRESERVE_STAGE=1
        fi
        if [ "$PRESERVE_STAGE" = 0 ]; then
            if [ -e "$STAGE/old-binary" ]; then
                mv -fT -- "$STAGE/old-binary" LabPanel || PRESERVE_STAGE=1
            fi
            if [ -d "$STAGE/old-dist" ]; then
                if [ -d frontend/dist ]; then
                    mv -T -- frontend/dist "$STAGE/failed-dist" || PRESERVE_STAGE=1
                fi
                mv -T -- "$STAGE/old-dist" frontend/dist || PRESERVE_STAGE=1
            fi
        fi
    fi
    if [ "$COMMITTED" = 0 ] && [ "$SERVICE_STOPPED" = 1 ] && [ "$SERVICE_WAS_ACTIVE" = 1 ] && [ "$PRESERVE_STAGE" = 0 ]; then
        app_systemctl start "$APP_SERVICE" || PRESERVE_STAGE=1
    fi
    if [ "$PRESERVE_STAGE" = 1 ]; then
        log "需要人工恢复，保留现场: ${STAGE}"
        exit 1
    fi
    if [ -n "$STAGED_CHECKOUT" ] && [ -d "$STAGED_CHECKOUT" ]; then
        run_user git worktree remove --force "$STAGED_CHECKOUT" || log "保留构建目录: ${STAGED_CHECKOUT}"
    fi
    if [ -n "$STAGE" ] && [ ! -d "$STAGED_CHECKOUT" ]; then
        rm -rf -- "$STAGE"
    fi
    exit "$result"
}

main() {
    local script_dir git_dir branch remote branch_ref current target expected port
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$script_dir"
    detect_run_user "$script_dir"
    load_env
    port="${PORT:-8080}"
    if ! [[ "$APP_SERVICE" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.@-]*$ && "$port" =~ ^[0-9]{1,5}$ ]]; then
        log "无效的服务名称或端口"
        exit 1
    fi
    git_dir="$(run_user git rev-parse --path-format=absolute --git-common-dir)"
    run_user touch -- "$git_dir/labpanel-update.lock"
    exec 9<"$git_dir/labpanel-update.lock"
    if ! flock -n 9; then
        log "已有更新任务正在执行"
        exit 1
    fi
    if [ -n "$(run_user git status --porcelain --untracked-files=no)" ]; then
        log "工作区包含本地修改，请先处理后再更新"
        exit 1
    fi
    local previous_stage
    for previous_stage in .labpanel-release.*/; do
        if [ -e "${previous_stage}old-binary" ] || [ -e "${previous_stage}old-dist" ]; then
            log "发现未完成的版本切换，保留现场: ${previous_stage}"
            exit 1
        fi
    done
    branch="$(run_user git symbolic-ref --quiet --short HEAD)"
    remote="$(run_user git config --get "branch.${branch}.remote" || printf '%s' origin)"
    branch_ref="$(run_user git config --get "branch.${branch}.merge" || printf 'refs/heads/%s' "$branch")"
    current="$(run_user git rev-parse HEAD)"
    if [ ! -x LabPanel ] || [ ! -s frontend/dist/index.html ]; then
        log "缺少可回滚的已部署版本，请先完成安装"
        exit 1
    fi
    if [ "$START_SERVICES" = 1 ]; then
        if [ "$(app_systemctl show "$APP_SERVICE" --value -p WorkingDirectory)" != "$script_dir" ]; then
            log "${APP_SERVICE}.service 的工作目录与当前部署不一致"
            exit 1
        fi
    fi

    configure_network
    select_github_route
    log "获取目标提交"
    fetch_target "$remote" "$branch_ref"
    target="$(run_user git rev-parse 'FETCH_HEAD^{commit}')"
    expected="$(run_user git rev-parse --short=12 "$target")"
    if ! run_user git merge-base --is-ancestor "$current" "$target"; then
        log "本地提交领先或已与远端分叉，保留现有版本"
        exit 1
    fi
    if [ "$current" = "$target" ] && [ "$START_SERVICES" = 1 ] &&
        [[ "$(curl --noproxy '*' -fsS --max-time 2 "http://127.0.0.1:${port}/api/version" 2>/dev/null || true)" == *\"commit\":\""$expected"\"* ]]; then
        log "运行版本已是 ${expected}"
        return 0
    fi

    STAGE="$(run_user mktemp -d -- "${script_dir}/.labpanel-release.XXXXXXXX")"
    STAGED_CHECKOUT="$STAGE/source"
    trap finish_update EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM HUP
    run_user git worktree add --detach "$STAGED_CHECKOUT" "$target"
    log "构建 ${expected}"
    run_user env APP_COMMIT="$expected" OUTPUT_BINARY=LabPanel FRONTEND_OUT_DIR=dist bash "$STAGED_CHECKOUT/build.sh"
    test -x "$STAGED_CHECKOUT/LabPanel"
    test -s "$STAGED_CHECKOUT/frontend/dist/index.html"

    if [ "$(run_user git rev-parse HEAD)" != "$current" ] || [ -n "$(run_user git status --porcelain --untracked-files=no)" ]; then
        log "构建期间工作区发生变化，停止部署"
        exit 1
    fi
    if [ "$START_SERVICES" = 1 ]; then
        if app_systemctl is-active --quiet "$APP_SERVICE"; then
            SERVICE_WAS_ACTIVE=1
        fi
        SERVICE_STOPPED=1
        app_systemctl stop "$APP_SERVICE"
    fi
    log "切换构建产物"
    ARTIFACTS_SWITCHING=1
    mv -T -- LabPanel "$STAGE/old-binary"
    mv -T -- frontend/dist "$STAGE/old-dist"
    mv -T -- "$STAGED_CHECKOUT/LabPanel" LabPanel
    mv -T -- "$STAGED_CHECKOUT/frontend/dist" frontend/dist

    if [ "$START_SERVICES" = 1 ]; then
        app_systemctl start "$APP_SERVICE"
        if ! wait_for_version "$expected" "$port"; then
            log "新版本健康检查未通过: ${expected}"
            exit 1
        fi
    fi
    run_user git merge --ff-only "$target"
    COMMITTED=1
    log "更新完成: ${expected}"
}

main "$@"
