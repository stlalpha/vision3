# ViSiON/3 BBS Setup Script (Windows)
# Requires PowerShell 5.1 or later.

$ErrorActionPreference = "Stop"

Write-Host "=== ViSiON/3 BBS Setup Script ===" -ForegroundColor Cyan
Write-Host ""

$scriptRoot = $PSScriptRoot
if (-not $scriptRoot) { $scriptRoot = Get-Location }
Set-Location $scriptRoot

$MISSING_PREREQS = $false

Write-Host "Checking prerequisites..."
Write-Host ""

# Check for Go (PATH + common Windows install locations)
$goExe = $null
$goCmd = Get-Command go -ErrorAction SilentlyContinue
if ($goCmd) { $goExe = $goCmd.Source }
if (-not $goExe) {
    $goCandidates = @(
        "$env:ProgramFiles\Go\bin\go.exe",
        "${env:ProgramFiles(x86)}\Go\bin\go.exe",
        "C:\Go\bin\go.exe",
        "$env:LOCALAPPDATA\Programs\Go\bin\go.exe"
    )
    foreach ($c in $goCandidates) {
        if ($c -and (Test-Path -LiteralPath $c)) { $goExe = $c; break }
    }
}
if (-not $goExe) {
    Write-Host "[X] Go is not installed" -ForegroundColor Red
    Write-Host "  Install from: https://golang.org/dl/"
    Write-Host "  After installing, close and reopen the terminal (or restart) so PATH is updated."
    $MISSING_PREREQS = $true
} else {
    try {
        $goVersionOutput = & $goExe version 2>&1
        if ($LASTEXITCODE -ne 0) { throw "go version failed" }
        if ($goVersionOutput -match "go(\d+\.\d+\.\d+)") {
            $GO_VERSION = $Matches[1]
            $REQUIRED_VERSION = [version]"1.24.2"
            $parsedGo = [version]$GO_VERSION
            if ($parsedGo -ge $REQUIRED_VERSION) {
                Write-Host "[OK] Go $GO_VERSION (required: $REQUIRED_VERSION or higher)" -ForegroundColor Green
            } else {
                Write-Host "[X] Go $GO_VERSION found, but version $REQUIRED_VERSION or higher is required" -ForegroundColor Red
                $MISSING_PREREQS = $true
            }
        } else {
            Write-Host "[X] Could not parse Go version" -ForegroundColor Red
            $MISSING_PREREQS = $true
        }
    } catch {
        Write-Host "[X] Go is not installed or not runnable" -ForegroundColor Red
        Write-Host "  Install from: https://golang.org/dl/"
        $MISSING_PREREQS = $true
    }
}

# Check for Git
try {
    $gitVersion = & git --version 2>&1
    if ($LASTEXITCODE -ne 0) { throw "git not found" }
    Write-Host "[OK] $gitVersion" -ForegroundColor Green
} catch {
    Write-Host "[X] Git is not installed" -ForegroundColor Red
    Write-Host "  Install from: https://git-scm.com/download/win"
    $MISSING_PREREQS = $true
}

# Check for ssh-keygen (OpenSSH on Windows 10+)
try {
    $null = Get-Command ssh-keygen -ErrorAction Stop
    Write-Host "[OK] SSH client (ssh-keygen)" -ForegroundColor Green
} catch {
    Write-Host "[X] SSH client (ssh-keygen) is not installed" -ForegroundColor Red
    Write-Host "  Windows 10+: Settings > Apps > Optional features > Add OpenSSH Client"
    $MISSING_PREREQS = $true
}

# Copy sexyz.ini to bin/ if not present
if (Test-Path "templates\configs\sexyz.ini") {
    if (-not (Test-Path "bin\sexyz.ini")) {
        Write-Host "  Creating bin\sexyz.ini from template..."
        New-Item -ItemType Directory -Path "bin" -Force | Out-Null
        Copy-Item "templates\configs\sexyz.ini" "bin\sexyz.ini"
    }
}

# Check for sexyz
if (Test-Path "bin\sexyz.exe") {
    Write-Host "[OK] sexyz (Synchronet ZModem 8k) at bin\sexyz.exe" -ForegroundColor Green
    if (Test-Path "bin\sexyz.ini") {
        Write-Host "[OK] sexyz.ini configuration found" -ForegroundColor Green
    } else {
        Write-Host "[!] bin\sexyz.ini not found - sexyz will use defaults" -ForegroundColor Yellow
    }
} else {
    Write-Host "[!] sexyz not found at bin\sexyz.exe (required for file transfers)" -ForegroundColor Yellow
    Write-Host "  Build from source: https://gitlab.synchro.net/main/sbbs.git"
    Write-Host "  See documentation/file-transfer-protocols.md for build instructions"
    Write-Host "  Place the binary at bin\sexyz.exe"
}

Write-Host ""

if ($MISSING_PREREQS) {
    Write-Host "Error: Missing required prerequisites!" -ForegroundColor Red
    Write-Host "Please install the missing components listed above and run setup.ps1 again."
    Write-Host ""
    Write-Host "For detailed installation instructions, see: documentation\installation.md"
    exit 1
}

Write-Host "All prerequisites satisfied!" -ForegroundColor Green
Write-Host ""

# SSH host key
if (-not (Test-Path "configs\ssh_host_rsa_key")) {
    Write-Host "Generating SSH host key (RSA)..."
    New-Item -ItemType Directory -Path "configs" -Force | Out-Null
    & ssh-keygen -q -t rsa -b 4096 -f "configs\ssh_host_rsa_key" -N ''
    Write-Host "SSH host key generated."
} else {
    Write-Host "SSH host key already exists."
}

# Create directories
Write-Host "Creating directory structure..."
$dirs = @(
    "data\users",
    "data\files\general",
    "data\logs",
    "data\msgbases",
    "data\ftn\in", "data\ftn\secure_in", "data\ftn\temp_in", "data\ftn\temp_out",
    "data\ftn\out", "data\ftn\dupehist", "data\ftn\dloads", "data\ftn\dloads\pass",
    "configs",
    "bin",
    "scripts"
)
foreach ($d in $dirs) {
    New-Item -ItemType Directory -Path $d -Force | Out-Null
}
Write-Host "Directories created."

# Copy template config files
Write-Host "Setting up configuration files..."
$templateConfigs = Join-Path $scriptRoot "templates\configs"
$configsDir = Join-Path $scriptRoot "configs"
Get-ChildItem -Path $templateConfigs -Filter "*.json" -ErrorAction SilentlyContinue | ForEach-Object {
    $target = Join-Path $configsDir $_.Name
    if (-not (Test-Path $target)) {
        Write-Host "  Creating $($_.Name) from template..."
        Copy-Item $_.FullName $target
    } else {
        Write-Host "  $($_.Name) already exists, skipping."
    }
}
Get-ChildItem -Path $templateConfigs -Filter "*.txt" -ErrorAction SilentlyContinue | ForEach-Object {
    $target = Join-Path $configsDir $_.Name
    if (-not (Test-Path $target)) {
        Write-Host "  Creating $($_.Name) from template..."
        Copy-Item $_.FullName $target
    } else {
        Write-Host "  $($_.Name) already exists, skipping."
    }
}

# UTF-8 without BOM (Go's JSON decoder does not accept BOM)
$utf8NoBom = New-Object System.Text.UTF8Encoding $false
# Create initial data files
if (-not (Test-Path "data\oneliners.json")) {
    Write-Host "Creating empty oneliners.json..."
    [System.IO.File]::WriteAllText("$scriptRoot\data\oneliners.json", "[]", $utf8NoBom)
}

if (-not (Test-Path "data\users\users.json")) {
    Write-Host "Creating initial users.json with default sysop account..."
    $usersJson = @'
[
  {
    "id": 1,
    "username": "felonius",
    "passwordHash": "$2a$10$4BzeQ5Pgg6GT6ckfLtTJOuInTvQxXRSj0DETBGIL87SYG2hHpXbtO",
    "handle": "Felonius",
    "accessLevel": 255,
    "flags": "",
    "lastLogin": "0001-01-01T00:00:00Z",
    "timesCalled": 0,
    "lastBulletinRead": "0001-01-01T00:00:00Z",
    "realName": "System Operator",
    "phoneNumber": "",
    "createdAt": "2024-01-01T00:00:00Z",
    "validated": true,
    "filePoints": 0,
    "numUploads": 0,
    "timeLimit": 60,
    "privateNote": "",
    "current_msg_conference_id": 1,
    "current_msg_conference_tag": "LOCAL",
    "current_file_conference_id": 1,
    "current_file_conference_tag": "LOCAL",
    "group_location": "",
    "current_message_area_id": 1,
    "current_message_area_tag": "GENERAL",
    "current_file_area_id": 1,
    "current_file_area_tag": "GENERAL",
    "screenWidth": 80,
    "screenHeight": 24
  }
]
'@
    New-Item -ItemType Directory -Path "data\users" -Force | Out-Null
    [System.IO.File]::WriteAllText("$scriptRoot\data\users\users.json", $usersJson, $utf8NoBom)
}

if (-not (Test-Path "data\users\callhistory.json")) {
    Write-Host "Creating empty callhistory.json..."
    [System.IO.File]::WriteAllText("$scriptRoot\data\users\callhistory.json", "[]", $utf8NoBom)
}

if (-not (Test-Path "data\users\callnumber.json")) {
    Write-Host "Creating callnumber.json..."
    [System.IO.File]::WriteAllText("$scriptRoot\data\users\callnumber.json", "1", $utf8NoBom)
}

# Build binaries
Write-Host ""
Write-Host "Building ViSiON/3..."
$buildFailed = $false
@(
    @{ Name = "vision3"; Desc = "BBS server" },
    @{ Name = "helper"; Desc = "helper process" },
    @{ Name = "v3mail"; Desc = "mail processor" },
    @{ Name = "strings"; Desc = "strings editor" },
    @{ Name = "ue"; Desc = "user editor" }
) | ForEach-Object {
    $exe = $_.Name + ".exe"
    Write-Host "Building $($_.Name)..."
    & $goExe build -o $exe "./cmd/$($_.Name)"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Build failed ($($_.Name))!" -ForegroundColor Red
        $buildFailed = $true
    }
}
if ($buildFailed) { exit 1 }

Write-Host "Initializing JAM bases..."
& .\v3mail.exe stats --all --config configs --data data 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Host "JAM initialization failed!" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "=== Setup Complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "Default login: felonius / password"
Write-Host "IMPORTANT: Change the default password immediately!"
Write-Host ""
Write-Host "To start the BBS:"
Write-Host "  .\vision3.exe"
Write-Host "  or run .\build.ps1 to rebuild and then .\vision3.exe to start."
Write-Host ""
Write-Host "To connect:"
Write-Host '  ssh user@localhost -p 2222'
