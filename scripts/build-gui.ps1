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

Push-Location (Join-Path $repoRoot 'gui')
try {
    go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -s -m -clean -skipbindings -trimpath -o tmt-gui.exe -ldflags '-s -w'
    if ($LASTEXITCODE -ne 0) { throw "Wails GUI 构建失败" }
    $builtBinary = Join-Path (Get-Location) 'build\bin\tmt-gui.exe'
    if (-not (Test-Path -LiteralPath $builtBinary)) { throw "Wails 未生成 tmt-gui.exe" }
    Copy-Item -LiteralPath $builtBinary -Destination $outputPath -Force
}
finally {
    Pop-Location
}

$binary = Get-Item -LiteralPath $outputPath
$hash = Get-FileHash -LiteralPath $outputPath -Algorithm SHA256
Write-Host "已构建 $($binary.FullName)"
Write-Host "大小: $($binary.Length) bytes"
Write-Host "SHA256: $($hash.Hash)"
