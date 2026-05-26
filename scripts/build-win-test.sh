#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"

mkdir -p "$GOCACHE"

echo "==> Building Windows proxy"
npm run build:proxy:win

echo "==> Building PotPlayer bridge"
npm run build:potbridge:win

echo "==> Compiling TypeScript"
npm run compile

echo "==> Building Windows installer"
npx electron-builder --win --publish=never

echo "==> Done"
echo "Installer: $ROOT_DIR/release/FNMedia_2.6.1_win_x64.exe"
