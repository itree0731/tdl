param(
    [String]$Version,
    [Boolean]$Proxy = $False
)


$Owner = "iyear"
$Repo = "tdl"
$Project = "tmt"
$Location = "$Env:SystemDrive\tmt"

$ErrorActionPreference = "Stop"

# check if run as admin
if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]"Administrator"))
{
    Write-Host "Please run this script as Administrator" -ForegroundColor Red
    exit 1
}

# use proxy if argument is passed
$PROXY_PREFIX = ""
if ($Proxy)
{
    $PROXY_PREFIX = "https://mirror.ghproxy.com/"
    Write-Host "Using GitHub proxy: $PROXY_PREFIX" -ForegroundColor Blue
}

# Set download ARCH based on system architecture
$Arch = ""
switch ($env:PROCESSOR_ARCHITECTURE)
{
    "AMD64" {
        $Arch = "64bit"
    }
    "x86" {
        $Arch = "32bit"
    }
    "ARM" {
        $Arch = "arm64"
    }
    default {
        Write-Host "Unsupported system architecture: $env:PROCESSOR_ARCHITECTURE" -ForegroundColor Red
        exit 1
    }
}

# set version
if (!$Version)
{
    $Version = (Invoke-RestMethod -Uri "https://api.github.com/repos/$Owner/$Repo/releases/latest").tag_name
}
Write-Host "Target version: $Version" -ForegroundColor Blue

# build download URL
$URL = "${PROXY_PREFIX}https://github.com/$Owner/$Repo/releases/download/$Version/${Project}_Windows_$Arch.zip"
$ChecksumURL = "${PROXY_PREFIX}https://github.com/$Owner/$Repo/releases/download/$Version/${Project}_checksums.txt"
Write-Host "Downloading $Project from $URL" -ForegroundColor Blue

# download archive and checksums
Invoke-WebRequest -Uri $URL -OutFile "$Project.zip"
# test zip path
if (-not(Test-Path "$Project.zip"))
{
    Write-Host "Download $URL failed" -ForegroundColor Red
    exit 1
}
Invoke-WebRequest -Uri $ChecksumURL -OutFile "$Project-checksums.txt"

# verify sha256 checksum before extracting
$ArchiveName = "${Project}_Windows_$Arch.zip"
$Expected = (Select-String -Path "$Project-checksums.txt" -Pattern ("(^|\s)" + [regex]::Escape($ArchiveName) + "\s*$") |
    Select-Object -First 1).Line -split '\s+' | Select-Object -First 1
if (-not $Expected)
{
    Write-Host "Checksum for $ArchiveName not found in checksums file" -ForegroundColor Red
    exit 1
}
$Actual = (Get-FileHash -Path "$Project.zip" -Algorithm SHA256).Hash.ToLower()
if ($Actual -ne $Expected.ToLower())
{
    Write-Host "Checksum mismatch: expected $Expected, got $Actual" -ForegroundColor Red
    exit 1
}
Write-Host "Checksum verified" -ForegroundColor Green

# extract tmt.exe to $LOCATION, add to PATH and remove temporary files
Expand-Archive -Path "$Project.zip" -DestinationPath "$Location" -Force

# if $LOCATION has not been added to PATH yet, add it
$PathEnv = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)
if (-not($PathEnv -like "*$Location*"))
{
    Write-Host "Adding $Location to Path Environment variable..." -ForegroundColor Blue

    $NewPath = $PathEnv + ";$Location"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, [EnvironmentVariableTarget]::Machine)
    # update current process' PATH
    [Environment]::SetEnvironmentVariable("Path", $NewPath, [EnvironmentVariableTarget]::Process)

    Write-Host "Note: Updates to PATH might not be visible until you restart your terminal application or reboot machine" -ForegroundColor Yellow
}
# remove zip and checksums file
Remove-Item "$Project.zip"
Remove-Item "$Project-checksums.txt" -ErrorAction SilentlyContinue

# test if installation is successful, and print instructions
if (-not(Get-Command $Project -ErrorAction SilentlyContinue))
{
    Write-Host "Installation failed" -ForegroundColor Red
    exit 1
}

Write-Host "$Project installed successfully! Location: $Location" -ForegroundColor Green
Write-Host "Run '$Project' to get started" -ForegroundColor Green
Write-Host "TMT documentation: https://docs.iyear.me/tdl" -ForegroundColor Green
