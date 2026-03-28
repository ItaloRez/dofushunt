$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$releaseDir = Join-Path $root "build/release-windows"
$zipPath = Join-Path $root "build/dofhunt-windows.zip"

Write-Host "[1/4] Limpando release antiga..."
if (Test-Path $releaseDir) { Remove-Item -Recurse -Force $releaseDir }
New-Item -ItemType Directory -Path $releaseDir | Out-Null

Write-Host "[2/4] Build do executavel..."
go build -o (Join-Path $releaseDir "dofhunt.exe") -ldflags "-s -w -H=windowsgui -extldflags=-static" .

Write-Host "[3/4] Copiando arquivos runtime..."
Copy-Item -Force "$root/clues.json" "$releaseDir/clues.json"
if (Test-Path "$root/.env") {
	Copy-Item -Force "$root/.env" "$releaseDir/.env"
}
if (Test-Path "$root/.env.example") {
	Copy-Item -Force "$root/.env.example" "$releaseDir/.env.example"
}
Copy-Item -Recurse -Force "$root/install/windows" "$releaseDir/install-windows"

Write-Host "[4/4] Gerando zip..."
if (Test-Path $zipPath) { Remove-Item -Force $zipPath }
Compress-Archive -Path "$releaseDir/*" -DestinationPath $zipPath

Write-Host "Pronto: $zipPath"
Write-Host "Oriente o usuario final a executar install-windows/install-deps.bat antes de abrir o app."
