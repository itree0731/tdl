param(
    [String]$Version,
    [Boolean]$Proxy = $False
)


$Owner = "iyear"
$Repo = "tdl"
$Location = "$Env:SystemDrive\tdl"

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
$URL = "${PROXY_PREFIX}https://github.com/$Owner/$Repo/releases/download/$Version/${Repo}_Windows_$Arch.zip"
$ChecksumURL = "${PROXY_PREFIX}https://github.com/$Owner/$Repo/releases/download/$Version/${Repo}_checksums.txt"
Write-Host "Downloading $Repo from $URL" -ForegroundColor Blue

# download archive and checksums
Invoke-WebRequest -Uri $URL -OutFile "$Repo.zip"
# test zip path
if (-not(Test-Path "$Repo.zip"))
{
    Write-Host "Download $URL failed" -ForegroundColor Red
    exit 1
}
Invoke-WebRequest -Uri $ChecksumURL -OutFile "$Repo-checksums.txt"

# verify sha256 checksum before extracting
$ArchiveName = "${Repo}_Windows_$Arch.zip"
$Expected = (Select-String -Path "$Repo-checksums.txt" -Pattern ("(^|\s)" + [regex]::Escape($ArchiveName) + "\s*$") |
    Select-Object -First 1).Line -split '\s+' | Select-Object -First 1
if (-not $Expected)
{
    Write-Host "Checksum for $ArchiveName not found in checksums file" -ForegroundColor Red
    exit 1
}
$Actual = (Get-FileHash -Path "$Repo.zip" -Algorithm SHA256).Hash.ToLower()
if ($Actual -ne $Expected.ToLower())
{
    Write-Host "Checksum mismatch: expected $Expected, got $Actual" -ForegroundColor Red
    exit 1
}
Write-Host "Checksum verified" -ForegroundColor Green

# only extract tdl.exe to $LOCATION , add to PATH and remove zip file
Expand-Archive -Path "$Repo.zip" -DestinationPath "$Location" -Force

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
Remove-Item "$Repo.zip"
Remove-Item "$Repo-checksums.txt" -ErrorAction SilentlyContinue

# test if installation is successful, and print instructions
if (-not(Get-Command $Repo -ErrorAction SilentlyContinue))
{
    Write-Host "Installation failed" -ForegroundColor Red
    exit 1
}

Write-Host "$Repo installed successfully! Location: $Location" -ForegroundColor Green
Write-Host "Run '$Repo' to get started" -ForegroundColor Green
Write-Host "To get started with tdl, please visit https://docs.iyear.me/tdl" -ForegroundColor Green
