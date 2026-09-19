# Install mtrx without Go. Usage:
#   irm https://raw.githubusercontent.com/snhsish/mtrx/main/scripts/install.ps1 | iex
param(
  [string]$Version = $env:MTRX_VERSION,
  [string]$InstallDir = $env:MTRX_BINDIR
)
$ErrorActionPreference = "Stop"
$Repo = "snhsish/mtrx"

if (-not $Version) {
  $Version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
}
if (-not $Version) { throw "Could not resolve latest version (set MTRX_VERSION=vX.Y.Z)" }

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" } "ARM64" { "arm64" } Default { throw "Unsupported arch: $env:PROCESSOR_ARCHITECTURE" }
}
if (-not $InstallDir) { $InstallDir = "$env:LocalAppData\mtrx\bin" }

$Zip = "mtrx-windows-$Arch.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Zip"
$Tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $Tmp | Out-Null
try {
  Write-Host "Downloading mtrx $Version for windows/$Arch..."
  Invoke-WebRequest $Url -OutFile (Join-Path $Tmp $Zip)
  Expand-Archive (Join-Path $Tmp $Zip) -DestinationPath $Tmp -Force
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Move-Item (Join-Path $Tmp "mtrx-windows-$Arch.exe") (Join-Path $InstallDir "mtrx.exe") -Force
  Write-Host "Installed to $InstallDir\mtrx.exe"
  & (Join-Path $InstallDir "mtrx.exe") version
  if (($env:Path -split ";") -notcontains $InstallDir) {
    Write-Warning "$InstallDir is not on your PATH. Add it to use mtrx from anywhere."
  }
} finally {
  Remove-Item $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}
