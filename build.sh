#!/bin/bash

set -e

echo "开始构建 LabPanel..."

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

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

# 检查是否存在 pnpm-lock.yaml，如果不存在则安装依赖
if [ ! -f "pnpm-lock.yaml" ]; then
    echo "安装前端依赖..."
    pnpm install
fi

# 构建前端
echo "执行前端构建..."
pnpm build

cd ..

# 检查前端构建结果
if [ ! -d "frontend/dist" ]; then
    echo "错误: 前端构建失败，未找到 dist 目录"
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
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o LabPanel main.go

if [ ! -f "LabPanel" ]; then
    echo "错误: 后端构建失败"
    exit 1
fi

echo "后端构建完成"
echo ""
echo "构建成功！"
echo "可执行文件: $(pwd)/LabPanel"
echo "前端文件: $(pwd)/frontend/dist"
echo ""
echo "运行方式: ./LabPanel"

