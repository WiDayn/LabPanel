#!/bin/bash

set -euo pipefail

APP_NAME="${APP_NAME:-LabPanel}"
INSTALL_DIR="${INSTALL_DIR:-/opt/LabPanel}"
REPO_URL="${REPO_URL:-}"
APP_SERVICE="${APP_SERVICE:-labpanel}"
FRP_SERVICE="${FRP_SERVICE:-frpc}"
FRP_VERSION="${FRP_VERSION:-latest}"
GO_VERSION="${GO_VERSION:-1.22.12}"
PORT="${PORT:-8080}"
APP_TITLE="${APP_TITLE:-LabPanel 管理面板}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
JWT_SECRET="${JWT_SECRET:-}"
USE_FRP="${USE_FRP:-}"
FRP_MODE="${FRP_MODE:-}"
FRP_INSTALL_DIR="${FRP_INSTALL_DIR:-}"
FRPC_PATH="${FRPC_PATH:-}"
FRP_CONFIG_PATH="${FRP_CONFIG_PATH:-}"
FRP_SERVER_ADDR="${FRP_SERVER_ADDR:-your-frps.example.com}"
FRP_SERVER_PORT="${FRP_SERVER_PORT:-7000}"
FRP_AUTH_TOKEN="${FRP_AUTH_TOKEN:-change-me}"
FRP_WEB_ADDR="${FRP_WEB_ADDR:-127.0.0.1}"
FRP_WEB_PORT="${FRP_WEB_PORT:-7400}"
START_SERVICES="${START_SERVICES:-1}"
RUN_DIR="$(pwd)"
LABPANEL_URL=""
FRP_SERVICE_CREATED="否"
FRP_CONFIG_CREATED="否"

log() {
    echo "[install] $*"
}

fail() {
    echo "[install] 错误: $*" >&2
    exit 1
}

prompt_text() {
    local prompt default value
    prompt="$1"
    default="$2"
    read -r -p "${prompt} [${default}]: " value
    echo "${value:-$default}"
}

prompt_secret() {
    local prompt default value
    prompt="$1"
    default="$2"
    read -r -s -p "${prompt} [直接回车使用默认值]: " value
    echo >&2
    echo "${value:-$default}"
}

prompt_yes_no() {
    local prompt default answer default_label
    prompt="$1"
    default="$2"
    if [ "$default" = "y" ]; then
        default_label="Y/n"
    else
        default_label="y/N"
    fi

    while true; do
        read -r -p "${prompt} [${default_label}]: " answer
        answer="${answer:-$default}"
        case "$answer" in
            y|Y|yes|YES|Yes) return 0 ;;
            n|N|no|NO|No) return 1 ;;
            *) echo "请输入 y 或 n" ;;
        esac
    done
}

prompt_choice() {
    local prompt default answer
    prompt="$1"
    default="$2"
    while true; do
        read -r -p "${prompt} [${default}]: " answer
        answer="${answer:-$default}"
        case "$answer" in
            1|2) echo "$answer"; return ;;
            *) echo "请输入 1 或 2" ;;
        esac
    done
}

collect_inputs() {
    if [ ! -t 0 ]; then
        USE_FRP="${USE_FRP:-y}"
        case "$USE_FRP" in
            y|Y|yes|YES|Yes) USE_FRP="y" ;;
            n|N|no|NO|No) USE_FRP="n" ;;
            *) fail "USE_FRP 只能是 y 或 n" ;;
        esac

        if [ "$USE_FRP" = "n" ]; then
            FRP_MODE="none"
            FRPC_PATH=""
            FRP_CONFIG_PATH=""
            return
        fi

        case "${FRP_MODE:-install}" in
            1|custom) FRP_MODE="custom" ;;
            2|install) FRP_MODE="install" ;;
            *) fail "FRP_MODE 只能是 install/custom 或 1/2" ;;
        esac

        if [ "$FRP_MODE" = "custom" ]; then
            FRPC_PATH="${FRPC_PATH:-/usr/local/bin/frpc}"
            FRP_CONFIG_PATH="${FRP_CONFIG_PATH:-/etc/frp/frpc.toml}"
        else
            FRP_INSTALL_DIR="${FRP_INSTALL_DIR:-${RUN_DIR}/frp}"
            FRPC_PATH="${FRPC_PATH:-${FRP_INSTALL_DIR}/frpc}"
            FRP_CONFIG_PATH="${FRP_CONFIG_PATH:-${FRP_INSTALL_DIR}/frpc.toml}"
        fi
        return
    fi

    echo "LabPanel 安装向导"
    echo
    PORT="$(prompt_text "LabPanel 访问端口" "$PORT")"
    APP_TITLE="$(prompt_text "面板标题" "$APP_TITLE")"
    ADMIN_USERNAME="$(prompt_text "管理员账号" "$ADMIN_USERNAME")"
    ADMIN_PASSWORD="$(prompt_secret "管理员密码" "$ADMIN_PASSWORD")"

    if [ -z "$USE_FRP" ]; then
        if prompt_yes_no "是否需要配置 FRP 客户端" "y"; then
            USE_FRP="y"
        else
            USE_FRP="n"
        fi
    else
        case "$USE_FRP" in
            y|Y|yes|YES|Yes) USE_FRP="y" ;;
            n|N|no|NO|No) USE_FRP="n" ;;
            *) fail "USE_FRP 只能是 y 或 n" ;;
        esac
    fi

    if [ "$USE_FRP" = "y" ]; then
        echo
        echo "FRP 配置方式："
        echo "  1) 使用已有 frpc"
        echo "  2) 下载并安装新的 frpc"
        case "$FRP_MODE" in
            custom) choice_default="1" ;;
            install|"") choice_default="2" ;;
            *) choice_default="$FRP_MODE" ;;
        esac
        choice="$(prompt_choice "请选择" "$choice_default")"
        if [ "$choice" = "1" ]; then
            FRP_MODE="custom"
            FRPC_PATH="$(prompt_text "frpc 可执行文件路径" "${FRPC_PATH:-/usr/local/bin/frpc}")"
            FRP_CONFIG_PATH="$(prompt_text "frpc.toml 配置文件路径" "${FRP_CONFIG_PATH:-/etc/frp/frpc.toml}")"
            FRP_SERVICE="$(prompt_text "FRP systemd service 名称" "$FRP_SERVICE")"
            echo
            echo "提醒：请确认 ${FRP_CONFIG_PATH} 已启用 [webServer]，例如："
            echo "[webServer]"
            echo "addr = \"127.0.0.1\""
            echo "port = 7400"
            echo
        else
            FRP_MODE="install"
            FRP_INSTALL_DIR="$(prompt_text "FRP 安装目录" "${FRP_INSTALL_DIR:-${RUN_DIR}/frp}")"
            FRPC_PATH="${FRP_INSTALL_DIR}/frpc"
            FRP_CONFIG_PATH="${FRP_INSTALL_DIR}/frpc.toml"
            FRP_SERVER_ADDR="$(prompt_text "FRP 服务端地址 serverAddr" "$FRP_SERVER_ADDR")"
            FRP_SERVER_PORT="$(prompt_text "FRP 服务端端口 serverPort" "$FRP_SERVER_PORT")"
            FRP_AUTH_TOKEN="$(prompt_secret "FRP auth token" "$FRP_AUTH_TOKEN")"
            FRP_WEB_ADDR="$(prompt_text "FRP webServer addr" "$FRP_WEB_ADDR")"
            FRP_WEB_PORT="$(prompt_text "FRP webServer port" "$FRP_WEB_PORT")"
            FRP_SERVICE="$(prompt_text "FRP systemd service 名称" "$FRP_SERVICE")"
        fi
    else
        FRP_MODE="none"
        FRPC_PATH=""
        FRP_CONFIG_PATH=""
    fi
}

need_root() {
    if [ "$(id -u)" -ne 0 ]; then
        exec sudo -E bash "$0" "$@"
    fi
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        armv7l|armv7*) echo "arm" ;;
        *) fail "暂不支持的架构: $(uname -m)" ;;
    esac
}

version_ge_121() {
    local version major minor
    version="$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//;s/[^0-9.].*$//')" || return 1
    major="${version%%.*}"
    minor="${version#*.}"
    minor="${minor%%.*}"
    [ "${major:-0}" -gt 1 ] || { [ "${major:-0}" -eq 1 ] && [ "${minor:-0}" -ge 21 ]; }
}

install_base_packages() {
    log "安装基础依赖..."
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update
        apt-get install -y ca-certificates curl tar gzip git npm
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y ca-certificates curl tar gzip git npm
    elif command -v yum >/dev/null 2>&1; then
        yum install -y ca-certificates curl tar gzip git npm
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm ca-certificates curl tar gzip git npm
    else
        fail "未识别包管理器，请先安装 curl、tar、git、npm"
    fi
}

install_go() {
    if command -v go >/dev/null 2>&1 && version_ge_121; then
        log "Go 已安装: $(go version)"
        return
    fi

    local arch tarball url
    arch="$(detect_arch)"
    tarball="go${GO_VERSION}.linux-${arch}.tar.gz"
    url="https://go.dev/dl/${tarball}"

    log "安装 Go ${GO_VERSION}..."
    curl -fsSL "$url" -o "/tmp/${tarball}"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "/tmp/${tarball}"
    ln -sf /usr/local/go/bin/go /usr/local/bin/go
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
}

install_pnpm() {
    if command -v pnpm >/dev/null 2>&1; then
        log "pnpm 已安装: $(pnpm --version)"
        return
    fi

    log "安装 pnpm..."
    if command -v corepack >/dev/null 2>&1; then
        corepack enable
        corepack prepare pnpm@latest --activate
    else
        npm install -g pnpm
    fi
}

prepare_source() {
    local script_dir origin
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    if [ -z "$REPO_URL" ] && git -C "$script_dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        origin="$(git -C "$script_dir" remote get-url origin 2>/dev/null || true)"
        REPO_URL="${origin:-}"
    fi
    REPO_URL="${REPO_URL:-https://github.com/WiDayn/LabPanel.git}"

    if [ ! -d "$INSTALL_DIR/.git" ]; then
        log "拉取代码到 ${INSTALL_DIR}..."
        mkdir -p "$(dirname "$INSTALL_DIR")"
        git clone "$REPO_URL" "$INSTALL_DIR"
    else
        log "安装目录已存在，更新代码..."
        git -C "$INSTALL_DIR" pull --ff-only
    fi
}

ensure_env() {
    local env_file jwt
    env_file="${INSTALL_DIR}/.env"
    jwt="$JWT_SECRET"
    if [ -z "$jwt" ]; then
        jwt="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 48 || true)"
        jwt="${jwt:-change-this-secret}"
    fi

    touch "$env_file"
    chmod 600 "$env_file"

    set_env_value "$env_file" "PORT" "$PORT"
    set_env_value "$env_file" "APP_TITLE" "$APP_TITLE"
    set_env_value "$env_file" "JWT_SECRET" "$jwt"
    set_env_value "$env_file" "ADMIN_USERNAME" "$ADMIN_USERNAME"
    set_env_value "$env_file" "ADMIN_PASSWORD" "$ADMIN_PASSWORD"
    set_env_value "$env_file" "TOML_PATH" "$FRP_CONFIG_PATH"
    set_env_value "$env_file" "SERVICE_NAME" "$FRP_SERVICE"
    set_env_value "$env_file" "FRPC_PATH" "$FRPC_PATH"
    set_env_value "$env_file" "DOCS_PATH" "${INSTALL_DIR}/docs"
    set_env_value "$env_file" "UPLOAD_PATH" "${INSTALL_DIR}/uploads"
    set_env_value "$env_file" "LXC_IMAGE" "ubuntu:22.04"
    set_env_value "$env_file" "LXC_BACKUP_DIR" "${INSTALL_DIR}/backups"

    mkdir -p "${INSTALL_DIR}/docs" "${INSTALL_DIR}/uploads" "${INSTALL_DIR}/backups"
}

set_env_value() {
    local file key value
    file="$1"
    key="$2"
    value="$3"
    if grep -qE "^${key}=" "$file"; then
        sed -i "/^${key}=/d" "$file"
    fi
    printf '%s=%s\n' "$key" "$value" >> "$file"
}

build_app() {
    log "编译前后端..."
    bash "${INSTALL_DIR}/build.sh"
}

install_frp() {
    local arch tag version tarball url tmpdir
    arch="$(detect_arch)"

    if [ "$FRP_VERSION" = "latest" ]; then
        tag="$(curl -fsSL https://api.github.com/repos/fatedier/frp/releases/latest | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
    else
        tag="$FRP_VERSION"
    fi
    [ -n "$tag" ] || fail "无法获取 FRP 版本"

    version="${tag#v}"
    tarball="frp_${version}_linux_${arch}.tar.gz"
    url="https://github.com/fatedier/frp/releases/download/${tag}/${tarball}"
    tmpdir="$(mktemp -d)"

    log "下载 FRP ${tag} 到 ${FRP_INSTALL_DIR}..."
    curl -fsSL "$url" -o "${tmpdir}/${tarball}"
    tar -C "$tmpdir" -xzf "${tmpdir}/${tarball}"
    mkdir -p "$FRP_INSTALL_DIR"
    install -m 0755 "${tmpdir}/frp_${version}_linux_${arch}/frpc" "$FRPC_PATH"
    rm -rf "$tmpdir"
}

write_frp_config() {
    mkdir -p "$(dirname "$FRP_CONFIG_PATH")"
    if [ ! -f "$FRP_CONFIG_PATH" ]; then
        log "生成 ${FRP_CONFIG_PATH}..."
        cat >"$FRP_CONFIG_PATH" <<EOF
serverAddr = "${FRP_SERVER_ADDR}"
serverPort = ${FRP_SERVER_PORT}

[auth]
token = "${FRP_AUTH_TOKEN}"

[webServer]
addr = "${FRP_WEB_ADDR}"
port = ${FRP_WEB_PORT}
EOF
        chmod 600 "$FRP_CONFIG_PATH"
        FRP_CONFIG_CREATED="是"
    else
        log "${FRP_CONFIG_PATH} 已存在，跳过覆盖"
        FRP_CONFIG_CREATED="已存在，未覆盖"
    fi
}

write_services() {
    log "写入 systemd services..."
    if [ "$USE_FRP" = "y" ]; then
        cat >/etc/systemd/system/${FRP_SERVICE}.service <<EOF
[Unit]
Description=FRP Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${FRPC_PATH} -c ${FRP_CONFIG_PATH}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
        systemctl enable "$FRP_SERVICE"
        FRP_SERVICE_CREATED="是"
    fi

    if [ "$USE_FRP" = "y" ]; then
        cat >/etc/systemd/system/${APP_SERVICE}.service <<EOF
[Unit]
Description=LabPanel
After=network-online.target ${FRP_SERVICE}.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=-${INSTALL_DIR}/.env
ExecStart=${INSTALL_DIR}/${APP_NAME}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    else
        cat >/etc/systemd/system/${APP_SERVICE}.service <<EOF
[Unit]
Description=LabPanel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=-${INSTALL_DIR}/.env
ExecStart=${INSTALL_DIR}/${APP_NAME}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    fi

    systemctl daemon-reload
    systemctl enable "$APP_SERVICE"
}

start_services() {
    if [ "$START_SERVICES" = "1" ]; then
        log "启动 services..."
        if [ "$USE_FRP" = "y" ]; then
            systemctl restart "$FRP_SERVICE" || log "${FRP_SERVICE} 启动失败，请检查 ${FRP_CONFIG_PATH}"
        fi
        systemctl restart "$APP_SERVICE"
    else
        log "START_SERVICES=0，已跳过启动 services"
    fi
}

print_summary() {
    local host
    host="$(hostname -I 2>/dev/null | awk '{print $1}')"
    host="${host:-127.0.0.1}"
    LABPANEL_URL="http://${host}:${PORT}"

    echo
    echo "========== LabPanel 安装结果 =========="
    echo "安装目录: ${INSTALL_DIR}"
    echo "访问地址: ${LABPANEL_URL}"
    echo "本机地址: http://127.0.0.1:${PORT}"
    echo "管理员账号: ${ADMIN_USERNAME}"
    echo "管理员密码: ${ADMIN_PASSWORD}"
    echo "配置文件: ${INSTALL_DIR}/.env"
    echo "应用 service: ${APP_SERVICE}.service"
    echo "应用 service 命令: systemctl status ${APP_SERVICE} --no-pager"
    echo "应用日志命令: journalctl -u ${APP_SERVICE} -f"
    echo "服务已启动: ${START_SERVICES}"
    echo
    if [ "$USE_FRP" = "y" ]; then
        echo "FRP 模式: ${FRP_MODE}"
        echo "frpc 路径: ${FRPC_PATH}"
        echo "FRP 配置: ${FRP_CONFIG_PATH}"
        echo "FRP 配置新建: ${FRP_CONFIG_CREATED}"
        echo "FRP webServer: ${FRP_WEB_ADDR}:${FRP_WEB_PORT}"
        echo "FRP service: ${FRP_SERVICE}.service"
        echo "FRP service 创建: ${FRP_SERVICE_CREATED}"
        echo "FRP service 命令: systemctl status ${FRP_SERVICE} --no-pager"
        if [ "$FRP_MODE" = "custom" ]; then
            echo "提醒: 使用已有 frpc 时，请确认配置文件已启用 [webServer]，否则面板无法热重载 FRP。"
        fi
    else
        echo "FRP: 未配置"
    fi
    echo "======================================="
}

main() {
    need_root "$@"
    collect_inputs
    install_base_packages
    install_go
    install_pnpm
    prepare_source
    ensure_env
    build_app
    if [ "$FRP_MODE" = "install" ]; then
        install_frp
        write_frp_config
    elif [ "$FRP_MODE" = "custom" ]; then
        [ -x "$FRPC_PATH" ] || log "提醒: ${FRPC_PATH} 不存在或不可执行，请安装后再启动 ${FRP_SERVICE}"
    fi
    write_services
    start_services
    print_summary
}

main "$@"
