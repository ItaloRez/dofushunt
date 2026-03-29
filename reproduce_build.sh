#!/bin/bash
set -e

# Configurar caminhos do pkg-config
export PKG_CONFIG_PATH="/mingw64/lib/pkgconfig"

echo "Aplicando truque do linker para Tesseract..."
# Copia as libs dinâmicas para extensões .a para enganar o linker (mesmo que no GitHub)
cp /mingw64/lib/libtesseract.dll.a /mingw64/lib/libtesseract.a || true
cp /mingw64/lib/libleptonica.dll.a /mingw64/lib/libleptonica.a || true

echo "Gerando recursos do Windows..."
go run github.com/tc-hib/go-winres@latest make

echo "Iniciando build Go..."
mkdir -p build-repro
go build -o build-repro/dofhunt.exe -ldflags "-s -w -H=windowsgui" .

echo "Agrupando dependências (.dll)..."
# Usa o ldd para encontrar todas as DLLs do MSYS2 necessárias e copia para a pasta do build
ldd build-repro/dofhunt.exe | grep /mingw64/bin/ | awk '{print $3}' | xargs -I '{}' cp '{}' build-repro/

echo "Copiando assets..."
cp clues.json build-repro/
[ -f .env ] && cp .env build-repro/
[ -d install ] && cp -r install build-repro/

echo "-----------------------------------"
echo "Build concluído em ./build-repro/"
echo "-----------------------------------"
