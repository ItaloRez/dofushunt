#!/bin/bash

set -e

ARCH="${1:-arm64}"  # arm64 (Apple Silicon) ou amd64 (Intel)
OUT="dist/dofushunt-mac-$ARCH"

rm -rf dist/
mkdir -p "$OUT"

echo "Compilando para darwin/$ARCH..."
GOOS=darwin GOARCH="$ARCH" go build -ldflags "-s -w" -o "$OUT/dofhunt" .

echo "Copiando arquivos..."
cp .env "$OUT/.env"

echo "Zipando..."
cd dist
zip -r "dofushunt-mac-$ARCH.zip" "dofushunt-mac-$ARCH/"
cd ..

echo ""
echo "Pronto! dist/dofushunt-mac-$ARCH.zip gerado."
echo "Seu amigo precisa ter o Tesseract instalado: brew install tesseract tesseract-lang"
