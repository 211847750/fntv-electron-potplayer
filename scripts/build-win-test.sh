#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
APP_VERSION="$(node -p "require('./package.json').version")"

mkdir -p "$GOCACHE"

echo "==> Preparing WinFsp runtime"
npm run prepare:winfsp

echo "==> Building Windows proxy"
npm run build:proxy:win

echo "==> Building PotPlayer bridge"
npm run build:potbridge:win

echo "==> Compiling TypeScript"
npm run compile

echo "==> Building Windows installer"
npx electron-builder --win --publish=never

echo "==> Done"
echo "Installer: $ROOT_DIR/release/FNMedia-PotPlayer_${APP_VERSION}_win_x64.exe"
