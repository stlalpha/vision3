# ViSiON/3 BBS Build Script (Windows)
#
# 1. First-time detection: Checks if essential setup files exist (SSH keys, user database)
# 2. Auto-setup: Runs setup.ps1 automatically if this is the first build
# 3. Build: Compiles the Go application from cmd/vision3 and other tools to the root directory
#
# Usage: .\build.ps1
# Then start the server with: .\vision3.exe

$ErrorActionPreference = "Stop"

$scriptRoot = $PSScriptRoot
if (-not $scriptRoot) { $scriptRoot = Get-Location }
Set-Location $scriptRoot

# First-time setup detection
if (-not (Test-Path "configs\ssh_host_rsa_key") -or -not (Test-Path "data\users\users.json")) {
    Write-Host "=== First-time setup detected ===" -ForegroundColor Cyan
    Write-Host "Running setup.ps1 first..."
    Write-Host ""
    & "$scriptRoot\setup.ps1"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Setup failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host ""
}

Write-Host "=== Building ViSiON/3 BBS ===" -ForegroundColor Cyan
$BUILT = @()

$goCmd = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCmd) {
    Write-Host "Go executable not found in PATH. Re-run setup.ps1 or reopen your shell." -ForegroundColor Red
    exit 1
}
$goExe = $goCmd.Source

$targets = @(
    @{ Cmd = "vision3"; Desc = "BBS server" },
    @{ Cmd = "helper"; Desc = "helper process" },
    @{ Cmd = "v3mail"; Desc = "mail processor" },
    @{ Cmd = "strings"; Desc = "strings editor" },
    @{ Cmd = "ue"; Desc = "user editor" },
    @{ Cmd = "config"; Desc = "config editor" }
)
foreach ($t in $targets) {
    $exe = $t.Cmd + ".exe"
    & $goExe build -o $exe "./cmd/$($t.Cmd)"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed ($($t.Cmd))!" -ForegroundColor Red
        exit 1
    }
    $BUILT += "  $exe - $($t.Desc)"
}

Write-Host "============================="
Write-Host "Build successful!" -ForegroundColor Green
Write-Host ""
foreach ($item in $BUILT) { Write-Host $item }
Write-Host ""
