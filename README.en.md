# LabPanel

[中文](README.md)

LabPanel is a lightweight control panel for personal servers, development boxes, and small lab environments. It brings FRP client management, LXC/LXD container operations, host metrics, GPU monitoring, and operational notes into one web interface.

It is designed for people who frequently expose local services through FRP, spin up throwaway containers, watch server resource usage, or keep deployment notes next to the tools they operate.

## Why LabPanel

- Manage FRP and LXC from one place instead of jumping between SSH sessions, TOML files, and ad-hoc notes.
- Built around real maintenance flows: service status, config edits, reloads, environment checks, and setup hints are connected in the UI.
- Lightweight deployment: a Go backend, a Vue frontend, and simple systemd integration.
- Friendly to personal servers: `.env` configuration, install/update scripts, and local-first data storage.
- No plaintext admin password at rest: passwords are stored as bcrypt hashes, and legacy plaintext `.env` values are migrated automatically.

## Features

### FRP Client Management

- View FRP service status.
- Edit base client settings.
- Manage proxy mappings in `frpc.toml`.
- Reload the service after saving configuration.
- Show actionable checks when FRP is missing or misconfigured.

### LXC/LXD Container Management

- Create, start, stop, force stop, restart, and delete containers.
- Edit container configuration and change the root password.
- Configure the default image and backup directory.
- Group containers for easier organization.
- Trigger backups, track backup status, and manage backup archives.

### Monitoring

- Host CPU, memory, disk, network, and process metrics.
- LXC container metrics.
- NVIDIA GPU usage, memory, temperature, and process ownership.
- Local SQLite metrics storage with configurable retention.

### Docs and Settings

- Maintain Markdown documentation inside the panel.
- Upload images for documentation.
- Change the panel title from the UI.
- Change the admin username and password from the UI.

## Tech Stack

- Backend: Go, Gin, JWT, SQLite
- Frontend: Vue 3, Vite, Tailwind CSS
- Configuration: `.env`, systemd, FRP TOML
- Runtime: Linux, systemd, FRP, LXC/LXD

## Quick Start

The recommended path is the install script. It pulls the project, installs required build tools, builds the frontend and backend, and creates systemd services.

```bash
curl -fsSL https://raw.githubusercontent.com/WiDayn/LabPanel/main/install.sh -o install.sh
sudo bash install.sh
```

If GitHub access is slow, use a proxy prefix:

```bash
curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/WiDayn/LabPanel/main/install.sh -o install.sh
sudo GITHUB_PROXY=https://gh-proxy.com/ bash install.sh
```

If you already cloned the repository:

```bash
sudo ./install.sh
```

The installer asks for the panel port, title, admin credentials, and whether FRP should be configured.

Default login:

```text
Username: admin
Password: admin
```

After the first login, change the admin account in System Settings and replace `JWT_SECRET` in `.env`.

## Unattended Install Example

```bash
sudo REPO_URL=https://github.com/WiDayn/LabPanel.git \
  INSTALL_DIR=/opt/LabPanel \
  PORT=8080 \
  APP_TITLE="LabPanel" \
  ADMIN_USERNAME=admin \
  ADMIN_PASSWORD=change-me \
  USE_FRP=y \
  FRP_MODE=install \
  FRP_SERVER_ADDR=your-frps.example.com \
  FRP_SERVER_PORT=7000 \
  FRP_AUTH_TOKEN=your-token \
  ./install.sh
```

A typical installation creates:

- `/opt/LabPanel/LabPanel`
- `/opt/LabPanel/.env`
- `lab-panel.service`
- `frpc`, `frpc.toml`, and `frpc.service` if FRP is enabled

## Updating

Run the update script from the install directory:

```bash
cd /opt/LabPanel
./update.sh
```

The script stops the service, pulls the latest code, rebuilds the app, and starts the service again. Running it as a normal user is recommended; the script calls `sudo` only when it needs to control systemd.

If your service names differ:

```bash
APP_SERVICE=lab-panel FRP_SERVICE=frpc ./update.sh
```

If GitHub access is slow:

```bash
GITHUB_PROXY=https://gh-proxy.com/ HTTPS_PROXY=http://127.0.0.1:7890 ./update.sh
```

## Configuration

LabPanel reads configuration from environment variables and `.env` in the project root. Start from `.env.example`:

```env
PORT=8080
APP_TITLE="LabPanel"
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

Important options:

- `APP_TITLE`: Browser title and header title.
- `JWT_SECRET`: JWT signing secret. Change it in production.
- `ADMIN_USERNAME`: Admin username.
- `ADMIN_HASHED_PASSWORD`: bcrypt hash of the admin password.
- `TOML_PATH`: Path to `frpc.toml`.
- `FRPC_PATH`: Path to the `frpc` executable.
- `LXC_IMAGE`: Default image for new containers.
- `METRICS_RETENTION_DAYS`: Metrics retention period. Use `0` to disable automatic cleanup.

If an older `.env` still contains `ADMIN_PASSWORD`, LabPanel migrates it to `ADMIN_HASHED_PASSWORD` on startup and removes the plaintext value.

Generate a password hash manually:

```bash
./LabPanel hash-password 'your-new-password'
```

## FRP Requirements

LabPanel needs access to the `frpc` executable, the `frpc.toml` file, and the related systemd service. To let the panel reload FRP after saving configuration, enable the local web server in `frpc.toml`:

```toml
serverAddr = "your-frps.example.com"
serverPort = 7000

[auth]
token = "your-token"

[webServer]
addr = "127.0.0.1"
port = 7400
```

## LXC/LXD Requirements

LXD must be installed and initialized, and the user running LabPanel must be able to execute `lxc`.

Ubuntu/Debian example:

```bash
sudo apt-get update
sudo apt-get install -y snapd
sudo snap install lxd
sudo lxd init --auto
getent group lxd | grep -qwF "$USER" || sudo usermod -aG lxd "$USER"
newgrp lxd
```

Check the installation:

```bash
lxc version
lxc list
```

## Local Development

```bash
git clone https://github.com/WiDayn/LabPanel.git
cd LabPanel
cd frontend
pnpm install
pnpm dev
```

Start the backend in another terminal:

```bash
go run main.go
```

Build for production:

```bash
./build.sh
```

Build outputs:

- `./LabPanel`
- `./frontend/dist`

## Troubleshooting

### pnpm Requires a Newer Node.js

Node.js 18 works well with pnpm 9:

```bash
sudo npm install -g pnpm@9.15.9
```

If you use nvm, prefer `./update.sh` over `sudo ./update.sh` so the build step uses your user-level Node.js and pnpm.

### `packages field missing or empty`

Make sure `frontend/pnpm-workspace.yaml` contains:

```yaml
packages:
  - "."

allowBuilds:
  esbuild: false
```

### Cannot Remove `frontend/dist`

Old build artifacts may be owned by root:

```bash
sudo chown -R "$USER:$USER" frontend/dist
```

### FRP Is Not Ready

Check `FRPC_PATH`, `TOML_PATH`, and `FRP_SERVICE`, then verify:

```bash
/usr/local/bin/frpc verify -c /etc/frp/frpc.toml
systemctl status frpc --no-pager
```

### LXC Is Not Ready

Check that LXD is initialized and the current user belongs to the `lxd` group:

```bash
lxc version
lxc list
```

## Project Layout

```text
.
├── config/      # Configuration loading and .env migration
├── handlers/    # HTTP API handlers
├── service/     # FRP, LXC, metrics, docs, and system services
├── models/      # Request and response models
├── middleware/  # Authentication middleware
├── frontend/    # Vue 3 frontend
├── docs/        # Default documentation directory
├── build.sh     # Build script
├── install.sh   # Install script
└── update.sh    # Update script
```