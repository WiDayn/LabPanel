# LabPanel

LabPanel 是一个用于管理 FRP 客户端配置和 LXC 容器的面板项目。

后端使用 Go + Gin，前端使用 Vue 3 + Vite。项目支持：

- 查看和重载 `frpc` 服务状态
- 管理 `frpc.toml` 中的代理映射
- 管理 LXC/LXD 容器的创建、启动、停止、重启、删除
- 在 FRP 或 LXC 未安装时，通过 `/api/check` 返回缺失项与安装引导

## 环境要求

- Linux
- Go 1.21+
- Node.js 18+
- `pnpm`
- `systemd`

如果你要使用完整功能，还需要：

- FRP 客户端 `frpc`
- LXC/LXD

## 目录结构

- `main.go`: 后端入口
- `handlers/`: HTTP 接口
- `service/`: 业务逻辑
- `frontend/`: Vue 前端
- `config/`: 配置加载
- `build.sh`: 一键构建脚本

## 一键安装

脚本会自动安装 Go 和 pnpm，拉取/更新项目，编译前后端，并通过交互向导询问：

- LabPanel 访问端口
- 管理员账号与密码
- 是否需要配置 FRP
- 使用已有 `frpc`，还是下载并安装新的 `frpc`
- 新安装 FRP 时的安装目录，默认是当前执行目录下的 `frp`

如果选择使用已有 `frpc`，脚本会提醒你确认 `frpc.toml` 已启用 `[webServer]`，否则面板无法热重载 FRP。安装完成后，脚本会汇总输出访问地址、账号密码、FRP 路径、配置路径和 service 信息。

推荐在服务器上执行：

```bash
curl -fsSL https://raw.githubusercontent.com/WiDayn/LabPanel/main/install.sh -o install.sh
sudo bash install.sh
```

如果你已经克隆了本仓库，也可以直接运行：

```bash
sudo ./install.sh
```

常用安装参数可以通过环境变量传入：

```bash
sudo REPO_URL=https://github.com/WiDayn/LabPanel.git \
  INSTALL_DIR=/opt/LabPanel \
  PORT=8080 \
  ADMIN_USERNAME=admin \
  ADMIN_PASSWORD=change-me \
  USE_FRP=y \
  FRP_MODE=install \
  FRP_INSTALL_DIR="$PWD/frp" \
  FRP_SERVER_ADDR=your-frps.example.com \
  FRP_SERVER_PORT=7000 \
  FRP_AUTH_TOKEN=your-token \
  ./install.sh
```

默认会创建：

- `labpanel.service`: 启动 LabPanel
- `frpc.service`: 启动 FRP 客户端，只有选择配置 FRP 时创建
- `/opt/LabPanel/.env`: LabPanel 运行配置
- `<FRP_INSTALL_DIR>/frpc.toml`: 新安装 FRP 时生成的客户端配置

如果暂时不想启动服务：

```bash
sudo START_SERVICES=0 ./install.sh
```

使用已有 `frpc` 的非交互示例：

```bash
sudo USE_FRP=y \
  FRP_MODE=custom \
  FRPC_PATH=/usr/local/bin/frpc \
  FRP_CONFIG_PATH=/etc/frp/frpc.toml \
  ./install.sh
```

不配置 FRP：

```bash
sudo USE_FRP=n ./install.sh
```

## 更新

更新脚本会停止 `labpanel` 和 `frpc`，执行 `git pull --ff-only`，重新编译前后端，然后启动服务：

```bash
cd /opt/LabPanel
sudo ./update.sh
```

如果你的 service 名称不同：

```bash
sudo APP_SERVICE=labpanel FRP_SERVICE=frpc ./update.sh
```

## 快速开始

### 1. 安装基础依赖

Ubuntu / Debian:

```bash
sudo apt-get update
sudo apt-get install -y golang nodejs npm
sudo npm install -g pnpm
```

CentOS / Rocky / AlmaLinux / Fedora:

```bash
sudo dnf install -y golang nodejs npm
sudo npm install -g pnpm
```

### 2. 克隆并安装前端依赖

```bash
git clone <your-repo-url>
cd LabPanel
cd frontend
pnpm install
cd ..
```

### 3. 配置环境变量

项目支持 `.env` 文件。可以在仓库根目录创建：

```env
PORT=8080
APP_TITLE=LabPanel 管理面板
JWT_SECRET=change-me
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin123

APP_SERVICE=labpanel
FRP_SERVICE=frpc
TOML_PATH=/etc/frp/frpc.toml
FRPC_PATH=/usr/local/bin/frpc

DOCS_PATH=./docs
UPLOAD_PATH=./uploads
LXC_IMAGE=ubuntu:22.04
```

说明：

- `PORT`: 后端监听端口
- `APP_TITLE`: 面板标题，同时用于浏览器 title 和顶部标题
- `JWT_SECRET`: 登录签名密钥
- `ADMIN_USERNAME`: 面板登录用户名
- `ADMIN_PASSWORD`: 面板登录密码
- `APP_SERVICE`: `systemd` 中的 LabPanel 服务名
- `FRP_SERVICE`: `systemd` 中的 FRP 服务名
- `TOML_PATH`: `frpc.toml` 配置文件路径
- `FRPC_PATH`: `frpc` 可执行文件路径
- `DOCS_PATH`: 文档目录
- `UPLOAD_PATH`: 上传目录
- `LXC_IMAGE`: 新建 LXC 容器时使用的默认镜像

如果没有设置 `.env`，程序会使用默认值。

## 安装 FRP

LabPanel 依赖以下 FRP 组件：

- `frpc` 可执行文件
- `frpc.toml` 配置文件
- `systemd` 服务

### Ubuntu / Debian 示例

```bash
sudo apt-get update
sudo apt-get install -y curl tar
sudo mkdir -p /etc/frp
```

下载官方 `frp` 发布包后，将 `frpc` 安装到系统路径：

```bash
sudo cp frpc /usr/local/bin/frpc
sudo chmod +x /usr/local/bin/frpc
```

创建配置文件：

```bash
sudo editor /etc/frp/frpc.toml
```

示例配置：

```toml
serverAddr = "your-frps.example.com"
serverPort = 7000

[auth]
token = "your-token"

[webServer]
addr = "127.0.0.1"
port = 7400
```

创建 `systemd` 服务：

```bash
sudo editor /etc/systemd/system/frpc.service
```

示例服务文件：

```ini
[Unit]
Description=FRP Client
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/frpc -c /etc/frp/frpc.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用并启动：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now frpc
sudo systemctl status frpc --no-pager
```

## 安装 LXC / LXD

### Ubuntu / Debian

很多较新的 Ubuntu / Debian 环境里，`lxd` 和 `lxd-client` 已经不能直接通过 `apt install` 获取。推荐使用官方文档当前主推的 `snap` 安装方式。

先安装 `snapd`：

```bash
sudo apt-get update
sudo apt-get install -y snapd
```

再安装 LXD：

```bash
sudo snap install lxd
sudo lxd init --auto
```

给当前用户授予访问权限：

```bash
getent group lxd | grep -qwF "$USER" || sudo usermod -aG lxd "$USER"
newgrp lxd
```

安装完成后确认：

```bash
lxc version
```

### 其他发行版

请按当前发行版安装 `lxc` 或 `lxd`，并确保：

- `lxc` 命令可直接执行
- LXD 服务已初始化

## 开发运行

### 启动后端

```bash
go run main.go
```

默认监听：

```text
http://127.0.0.1:8080
```

### 启动前端开发服务器

```bash
cd frontend
pnpm dev
```

## 构建

项目自带构建脚本：

```bash
chmod +x build.sh
./build.sh
```

构建完成后会生成：

- `./LabPanel`: 后端可执行文件
- `./frontend/dist`: 前端静态文件

也可以手动构建：

```bash
cd frontend
pnpm install
pnpm build
cd ..
go mod download
go build -o LabPanel main.go
```

## 运行检查机制

后端提供：

```text
GET /api/check
```

该接口会检查：

- FRP 是否已安装
- `frpc.toml` 是否存在
- FRP 的 `systemd` 服务是否存在
- `lxc` 命令是否可用

当前端检测到 FRP 或 LXC 未就绪时，会直接展示安装指引，而不是只显示命令执行失败的报错。

## 登录

默认登录信息：

```text
用户名：admin
密码：admin123
```

生产环境请务必修改：

- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `JWT_SECRET`

## 常见问题

### 1. 页面提示 FRP 未就绪

请检查：

- `FRPC_PATH` 是否正确
- `TOML_PATH` 指向的配置文件是否存在
- `FRP_SERVICE` 对应的服务是否已创建

可手动验证：

```bash
/usr/local/bin/frpc verify -c /etc/frp/frpc.toml
systemctl status frpc --no-pager
```

### 2. 页面提示 LXC 未就绪

请检查：

- `lxc` 命令是否存在
- `lxd` 是否已初始化

可手动验证：

```bash
lxc version
lxc list
```

如果你在安装阶段看到下面这种错误：

```text
E: 软件包 lxd 没有可安装候选
E: 无法定位软件包 lxd-client
```

通常表示当前发行版的软件源不再提供旧的 `apt` 包。请改用：

```bash
sudo apt-get install -y snapd
sudo snap install lxd
sudo lxd init --auto
```

### 3. 构建失败，提示没有 `pnpm`

安装命令：

```bash
npm install -g pnpm
```

## 许可证

如果你准备开源，可以在这里补充许可证说明。
