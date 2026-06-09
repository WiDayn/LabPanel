#!/bin/bash

set -e

echo "开始构建 LabPanel..."

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

OUTPUT_BINARY="${OUTPUT_BINARY:-LabPanel}"
FRONTEND_OUT_DIR="${FRONTEND_OUT_DIR:-dist}"

use_sudo_user_node() {
    local sudo_user_home node_bin

    if [ "$(id -u)" -ne 0 ] || [ -z "${SUDO_USER:-}" ] || [ "$SUDO_USER" = "root" ]; then
        return
    fi

    sudo_user_home="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
    if [ -z "$sudo_user_home" ]; then
        return
    fi

    if [ -d "$sudo_user_home/.nvm/versions/node" ]; then
        node_bin="$(
            find "$sudo_user_home/.nvm/versions/node" \
                -mindepth 2 -maxdepth 2 -type f -path '*/bin/node' \
                -printf '%h\n' | sort -V | tail -n 1
        )"
        if [ -n "$node_bin" ]; then
            export PATH="$node_bin:$PATH"
        fi
    fi

    if [ -d "$sudo_user_home/.local/share/pnpm" ]; then
        export PNPM_HOME="$sudo_user_home/.local/share/pnpm"
        export PATH="$PNPM_HOME:$PATH"
    fi
}

use_sudo_user_node

# 检查 pnpm 是否安装
if ! command -v pnpm &> /dev/null; then
    echo "错误: 未找到 pnpm，请先安装 pnpm"
    echo "安装命令: npm install -g pnpm"
    exit 1
fi

# 检查 go 是否安装
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 go，请先安装 Go"
    exit 1
fi

# 构建前端
echo "正在构建前端..."
cd frontend

echo "安装前端依赖..."
if [ -f "pnpm-lock.yaml" ]; then
    pnpm install --frozen-lockfile
else
    pnpm install
fi

# 构建前端
echo "执行前端构建..."
if [ "$FRONTEND_OUT_DIR" = "dist" ]; then
    pnpm build
else
    pnpm exec vite build --outDir "$FRONTEND_OUT_DIR" --emptyOutDir
fi

cd ..

# 检查前端构建结果
if [ ! -d "frontend/$FRONTEND_OUT_DIR" ]; then
    echo "错误: 前端构建失败，未找到 frontend/$FRONTEND_OUT_DIR 目录"
    exit 1
fi

echo "前端构建完成"

# 构建后端
echo "正在构建后端..."

# 下载 Go 依赖
echo "下载 Go 依赖..."
go mod download
go mod tidy

# 构建可执行文件
echo "编译 Go 程序..."
GOARCH_VALUE="${GOARCH:-$(go env GOARCH)}"
APP_VERSION="${APP_VERSION:-$(node -e "console.log(require('./frontend/package.json').version)" 2>/dev/null || echo "1.0.0")}"
APP_COMMIT="${APP_COMMIT:-$(git rev-parse --short=12 HEAD 2>/dev/null || echo "unknown")}"
APP_COMMIT_DATE="${APP_COMMIT_DATE:-$(git show -s --format=%cs HEAD 2>/dev/null || date +%F)}"
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH_VALUE" go build \
    -ldflags="-w -s -X LabPanel/service.AppVersion=${APP_VERSION} -X LabPanel/service.AppCommit=${APP_COMMIT} -X LabPanel/service.AppCommitDate=${APP_COMMIT_DATE}" \
    -o "$OUTPUT_BINARY" main.go

if [ ! -f "$OUTPUT_BINARY" ]; then
    echo "错误: 后端构建失败，未找到 $OUTPUT_BINARY"
    exit 1
fi

echo "后端构建完成"
echo ""
echo "构建成功！"
echo "可执行文件: $(pwd)/$OUTPUT_BINARY"
echo "前端文件: $(pwd)/frontend/$FRONTEND_OUT_DIR"
echo ""
echo "运行方式: ./LabPanel"
