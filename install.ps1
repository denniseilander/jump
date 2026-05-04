#Requires -Version 5.1
[CmdletBinding()]
param (
    [string]$InstallDir = "$env:USERPROFILE\bin"
)

$ErrorActionPreference = 'Stop'
$Repo   = "denniseilander/jump"
$Binary = "jump.exe"

# Detect architecture
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
$goArch = switch ($arch) {
    'X64'   { 'amd64' }
    'Arm64' { 'arm64' }
    default { Write-Error "Unsupported architecture: $arch"; exit 1 }
}

# Get latest release version
Write-Host "Fetching latest release..."
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
$version = $release.tag_name
if (-not $version) {
    Write-Error "Could not determine latest version."
    exit 1
}
$versionNum = $version.TrimStart('v')
Write-Host "Installing jump $version (windows/$goArch)..."

# Download and extract
$archive  = "jump_${versionNum}_windows_${goArch}.zip"
$url      = "https://github.com/$Repo/releases/download/$version/$archive"
$tmp      = Join-Path $env:TEMP ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    $zipPath = Join-Path $tmp $archive
    Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing
    Expand-Archive -Path $zipPath -DestinationPath $tmp

    # Ensure install dir exists
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    # Copy binary
    $src = Join-Path $tmp $Binary
    $dst = Join-Path $InstallDir $Binary
    Copy-Item -Path $src -Destination $dst -Force

    # Add InstallDir to user PATH if not already present
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$InstallDir", 'User')
        Write-Host "Added $InstallDir to PATH (restart your terminal to apply)."
    }

    Write-Host "jump $version installed to $dst"
    Write-Host "Run: jump --help"
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
