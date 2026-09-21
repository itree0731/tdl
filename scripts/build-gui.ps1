$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$outputPath = Join-Path $repoRoot 'tmt-gui.exe'
$running = Get-CimInstance Win32_Process | Where-Object {
    $_.Name -eq 'tmt-gui.exe' -and $_.ExecutablePath -eq $outputPath
}
if ($running) {
    throw "tmt-gui.exe 正在运行，请先关闭窗口再构建。"
}

Push-Location (Join-Path $repoRoot 'gui\frontend')
try {
    if (-not (Test-Path -LiteralPath 'node_modules')) {
        npm ci
        if ($LASTEXITCODE -ne 0) { throw "npm ci 失败" }
    }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
}
finally {
    Pop-Location
}

Push-Location $repoRoot
try {
    go build -tags production -trimpath -ldflags '-s -w -H windowsgui' -o $outputPath ./gui
    if ($LASTEXITCODE -ne 0) { throw "GUI Go 构建失败" }
}
finally {
    Pop-Location
}

$binary = Get-Item -LiteralPath $outputPath
$hash = Get-FileHash -LiteralPath $outputPath -Algorithm SHA256
Write-Host "已构建 $($binary.FullName)"
Write-Host "大小: $($binary.Length) bytes"
Write-Host "SHA256: $($hash.Hash)"
