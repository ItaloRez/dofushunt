#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RELEASE_DIR="$ROOT_DIR/build/release-macos"
ZIP_PATH="$ROOT_DIR/build/dofhunt-macos.zip"

echo "[1/4] Limpando release antiga..."
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

echo "[2/4] Build do executavel..."
go build -o "$RELEASE_DIR/dofhunt" .

echo "[3/4] Copiando arquivos runtime..."
cp "$ROOT_DIR/clues.json" "$RELEASE_DIR/clues.json"
if [ -f "$ROOT_DIR/.env" ]; then
  cp "$ROOT_DIR/.env" "$RELEASE_DIR/.env"
fi
if [ -f "$ROOT_DIR/.env.example" ]; then
  cp "$ROOT_DIR/.env.example" "$RELEASE_DIR/.env.example"
fi
cp -R "$ROOT_DIR/install/macos" "$RELEASE_DIR/install-macos"
chmod +x "$RELEASE_DIR/install-macos/install.command"

echo "[4/4] Gerando zip..."
rm -f "$ZIP_PATH"
(
  cd "$RELEASE_DIR"
  zip -r "$ZIP_PATH" .
)

echo "Pronto: $ZIP_PATH"
echo "Oriente o usuario final a executar install-macos/install.command antes de abrir o app."
