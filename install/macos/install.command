#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

echo "======================================"
echo " DofHunt - Instalador de dependencias"
echo "======================================"

echo
echo "[1/3] Verificando Homebrew..."
if ! command -v brew >/dev/null 2>&1; then
  echo "Homebrew nao encontrado. Instalando Homebrew..."
  NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

echo
echo "[2/3] Instalando Tesseract e Leptonica..."
brew update
brew install tesseract leptonica

echo
echo "[3/3] Pronto. Dependencias instaladas com sucesso."
echo "Agora voce ja pode abrir o app normalmente."

echo
read -r -p "Pressione ENTER para fechar..." _
