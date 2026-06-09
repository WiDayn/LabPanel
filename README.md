# LabPanel

[English](README.en.md)

LabPanel 是一个为个人服务器、开发机和小型实验室环境准备的轻量管理面板。它把 FRP 客户端、LXC/LXD 容器、主机监控、GPU 监控和运维文档放进同一个 Web 界面，让一台服务器的常用维护工作不再散落在终端命令、配置文件和临时笔记里。

它适合这样的场景：你有一台或几台 Linux 主机，需要经常调整内网穿透端口、创建测试容器、观察资源占用，或者给自己和团队留下一份随手可查的部署文档。

## 为什么用 LabPanel

- 一个入口管理 FRP 和 LXC：常见操作直接在面板完成，减少反复登录服务器编辑文件。
- 面向真实运维流程：服务状态、配置编辑、重载、环境检查、安装提示都在界面里串起来。
- 轻量部署：Go 后端加 Vue 前端，构建后就是一个二进制和一份静态资源。
- 对个人服务器友好：默认支持 systemd、`.env` 配置、脚本安装和脚本更新。
- 密码不明文落盘：管理员密码使用 bcrypt 哈希存储，旧版明文配置会自动迁移。

## 功能概览

### FRP 客户端管理

- 查看 FRP 服务状态。
- 编辑基础连接配置。
- 管理 `frpc.toml` 中的代理映射。
- 保存配置后重载服务。
- FRP 未安装或配置缺失时显示检查结果和处理指引。

### LXC/LXD 容器管理

- 创建、启动、停止、强制停止、重启、删除容器。
- 修改容器配置和 root 密码。
- 配置默认镜像和备份目录。
- 容器分组管理。
- 触发备份、查看备份状态、管理备份归档。

### 监控和观察

- 主机 CPU、内存、磁盘、网络和进程信息。
- LXC 容器资源指标。
- NVIDIA GPU 使用率、显存、温度和进程占用。
- 监控数据使用本地 SQLite 存储，并支持保留天数配置。

### 文档和系统设置

- 在面板内维护 Markdown 文档。
- 支持文档图片上传。
- 在线修改系统标题。
- 在线修改管理员账号和密码。

## 技术栈

- 后端：Go、Gin、JWT、SQLite
- 前端：Vue 3、Vite、Tailwind CSS
- 配置：`.env`、systemd、FRP TOML
- 运行环境：Linux、systemd、FRP、LXC/LXD

## 快速开始

推荐使用安装脚本，它会拉取代码、安装必要依赖、构建前后端，并创建 systemd 服务。

```bash
curl -fsSL https://raw.githubusercontent.com/WiDayn/LabPanel/main/install.sh -o install.sh
sudo bash install.sh
```

如果访问 GitHub 较慢，可以使用代理前缀：

```bash
curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/WiDayn/LabPanel/main/install.sh -o install.sh
sudo GITHUB_PROXY=https://gh-proxy.com/ bash install.sh
```

已经克隆仓库时：

```bash
sudo ./install.sh
```

安装向导会询问访问端口、面板标题、管理员账号密码，以及是否配置 FRP。

默认登录信息：

```text
用户名：admin
密码：admin
```

首次部署后请在“系统设置”里修改账号密码，并更换 `.env` 中的 `JWT_SECRET`。

## 非交互安装示例

```bash
sudo REPO_URL=https://github.com/WiDayn/LabPanel.git \
  INSTALL_DIR=/opt/LabPanel \
  PORT=8080 \
  APP_TITLE="LabPanel 管理面板" \
  ADMIN_USERNAME=admin \
  ADMIN_PASSWORD=change-me \
  USE_FRP=y \
  FRP_MODE=install \
  FRP_SERVER_ADDR=your-frps.example.com \
  FRP_SERVER_PORT=7000 \
  FRP_AUTH_TOKEN=your-token \
  ./install.sh
```

常见安装结果：

- `/opt/LabPanel/LabPanel`
- `/opt/LabPanel/.env`
- `lab-panel.service`
- 如果选择配置 FRP，还会创建或复用 `frpc`、`frpc.toml` 和 `frpc.service`

## 更新

在安装目录运行：

```bash
cd /opt/LabPanel
./update.sh
```

更新脚本会停止服务、拉取最新代码、重新构建并启动服务。推荐使用普通用户运行；脚本会在需要控制 systemd 时自动调用 `sudo`。

如果服务名不同：

```bash
APP_SERVICE=lab-panel FRP_SERVICE=frpc ./update.sh
```

如果服务器访问 GitHub 较慢：

```bash
GITHUB_PROXY=https://gh-proxy.com/ HTTPS_PROXY=http://127.0.0.1:7890 ./update.sh
```

## 配置

LabPanel 从环境变量和项目根目录的 `.env` 加载配置。可以复制 `.env.example` 后按需修改。

```env
PORT=8080
APP_TITLE="LabPanel 管理面板"
JWT_SECRET=your-secret-key-change-in-production

ADMIN_USERNAME=admin
ADMIN_HASHED_PASSWORD="\$2a\$10\$6DCHyW8VUR/0WV8RwtIRDuHlpK26WKHTVark3IWtTl3djv4oNkoIW"

APP_SERVICE=lab-panel
FRP_SERVICE=frpc
TOML_PATH=/etc/frp/frpc.toml
FRPC_PATH=/usr/local/bin/frpc

DOCS_PATH=./docs
UPLOAD_PATH=./uploads
LXC_BACKUP_DIR=./backups
LXC_GROUPS_PATH=./lxc_groups.json
METRICS_DB_PATH=./data/metrics.db
METRICS_RETENTION_DAYS=30
LXC_IMAGE=ubuntu:22.04
```

重要配置项：

- `APP_TITLE`: 浏览器标题和面板顶部标题。
- `JWT_SECRET`: 登录令牌签名密钥，生产环境必须修改。
- `ADMIN_USERNAME`: 管理员用户名。
- `ADMIN_HASHED_PASSWORD`: 管理员密码的 bcrypt 哈希。
- `TOML_PATH`: `frpc.toml` 路径。
- `FRPC_PATH`: `frpc` 可执行文件路径。
- `LXC_IMAGE`: 创建容器时使用的默认镜像。
- `METRICS_RETENTION_DAYS`: 监控数据保留天数，`0` 表示不自动清理。

旧版 `.env` 如果仍包含 `ADMIN_PASSWORD`，LabPanel 启动时会自动生成 `ADMIN_HASHED_PASSWORD` 并删除明文密码。

手动生成密码哈希：

```bash
./LabPanel hash-password 'your-new-password'
```

## FRP 要求

LabPanel 需要能访问 `frpc` 可执行文件、`frpc.toml` 配置文件和对应 systemd 服务。为了让面板保存配置后可以热重载 FRP，推荐在 `frpc.toml` 中启用本地 webServer：

```toml
serverAddr = "your-frps.example.com"
serverPort = 7000

[auth]
token = "your-token"

[webServer]
addr = "127.0.0.1"
port = 7400
```

## LXC/LXD 要求

服务器需要安装并初始化 LXD，运行 LabPanel 的用户需要有权限执行 `lxc` 命令。

Ubuntu/Debian 示例：

```bash
sudo apt-get update
sudo apt-get install -y snapd
sudo snap install lxd
sudo lxd init --auto
getent group lxd | grep -qwF "$USER" || sudo usermod -aG lxd "$USER"
newgrp lxd
```

检查：

```bash
lxc version
lxc list
```

## 本地开发

```bash
git clone https://github.com/WiDayn/LabPanel.git
cd LabPanel
cd frontend
pnpm install
pnpm dev
```

另开一个终端启动后端：

```bash
go run main.go
```

构建生产版本：

```bash
./build.sh
```

构建产物：

- `./LabPanel`
- `./frontend/dist`

## 常见问题

### pnpm 提示 Node.js 版本过低

Node.js 18 推荐使用 pnpm 9：

```bash
sudo npm install -g pnpm@9.15.9
```

如果你使用 nvm 管理 Node，更推荐直接运行 `./update.sh`，不要用 `sudo ./update.sh`，这样构建阶段会使用当前用户的 Node 和 pnpm。

### pnpm 提示 `packages field missing or empty`

确认 `frontend/pnpm-workspace.yaml` 包含：

```yaml
packages:
  - "."

allowBuilds:
  esbuild: false
```

### 前端构建时无法删除 `frontend/dist`

通常是旧构建产物属于 root：

```bash
sudo chown -R "$USER:$USER" frontend/dist
```

### 页面提示 FRP 未就绪

检查 `.env` 中的 `FRPC_PATH`、`TOML_PATH`、`FRP_SERVICE`，并验证：

```bash
/usr/local/bin/frpc verify -c /etc/frp/frpc.toml
systemctl status frpc --no-pager
```

### 页面提示 LXC 未就绪

检查 LXD 是否初始化、当前用户是否属于 `lxd` 组：

```bash
lxc version
lxc list
```

## 目录结构

```text
.
├── config/      # 配置加载和 .env 迁移
├── handlers/    # HTTP API
├── service/     # FRP、LXC、监控、文档等业务逻辑
├── models/      # 请求和响应模型
├── middleware/  # 鉴权中间件
├── frontend/    # Vue 3 前端
├── docs/        # 默认文档目录
├── build.sh     # 构建脚本
├── install.sh   # 安装脚本
└── update.sh    # 更新脚本
```