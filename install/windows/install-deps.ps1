$ErrorActionPreference = 'Stop'

function Ensure-Admin {
    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
    if (-not $isAdmin) {
        Write-Host "Reiniciando instalador com permissao de administrador..."
        Start-Process -FilePath "powershell.exe" -Verb RunAs -ArgumentList "-ExecutionPolicy Bypass -File `"$PSCommandPath`""
        exit 0
    }
}

function Install-WithWinget {
    Write-Host "Tentando instalar Tesseract via winget..."
    winget install --id UB-Mannheim.TesseractOCR -e --silent --accept-source-agreements --accept-package-agreements
}

function Install-WithChoco {
    Write-Host "Tentando instalar Tesseract via Chocolatey..."
    choco install tesseract -y
}

Ensure-Admin

Write-Host "======================================"
Write-Host " DofHunt - Instalador de dependencias"
Write-Host "======================================"
Write-Host ""

$winget = Get-Command winget -ErrorAction SilentlyContinue
$choco = Get-Command choco -ErrorAction SilentlyContinue

if ($winget) {
    Install-WithWinget
}
elseif ($choco) {
    Install-WithChoco
}
else {
    Write-Host "Nem winget nem choco encontrados."
    Write-Host "Instale um deles e rode novamente:"
    Write-Host "- winget (recomendado): atualize o App Installer da Microsoft Store"
    Write-Host "- choco: https://chocolatey.org/install"
    exit 1
}

Write-Host ""
Write-Host "Dependencias instaladas com sucesso."
Write-Host "Voce ja pode abrir o DofHunt."
Read-Host "Pressione ENTER para fechar"
