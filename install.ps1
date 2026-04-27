#!/usr/bin/env pwsh
# mcp-core Windows installer
param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\mcp-core"
)

$Repo = "0xnu/mcp-core"

if ($Version -eq "latest") {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $release.tag_name
}

$Arch = if ([Environment]::ProcessorArchitecture -eq 'Arm64') { "arm64" } else { "amd64" }
$ArchiveVersion = $Version.TrimStart('v')
$ZipName = "mcp-core-${ArchiveVersion}-windows-${Arch}.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$ZipName"

Write-Host "Downloading mcp-core $Version for windows/$Arch..." -ForegroundColor Green

$TempDir = Join-Path $env:TEMP "mcp-core-install"
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
$ZipPath = Join-Path $TempDir $ZipName

try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force

    # Rename to plain names without platform suffix
    $hubOld = Join-Path $InstallDir "mcp-core-windows-${Arch}.exe"
    $hubNew = Join-Path $InstallDir "mcp-core.exe"
    if (Test-Path $hubOld) { Move-Item -Force $hubOld $hubNew }

    $ctlOld = Join-Path $InstallDir "corectl-windows-${Arch}.exe"
    $ctlNew = Join-Path $InstallDir "corectl.exe"
    if (Test-Path $ctlOld) { Move-Item -Force $ctlOld $ctlNew }

    # Add to PATH if not already present
    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($currentPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to your PATH (user scope)" -ForegroundColor Yellow
        Write-Host "You may need to restart your terminal for this to take effect." -ForegroundColor Yellow
    }

    Write-Host "Installed mcp-core and corectl to $InstallDir" -ForegroundColor Green
    Write-Host "Run 'mcp-core' to start the daemon" -ForegroundColor Green
}
catch {
    Write-Error "Installation failed: $_"
    exit 1
}
finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
