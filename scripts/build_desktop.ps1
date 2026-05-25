$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$desktopDir = Join-Path $repoRoot 'app\desktop'
$wails = Join-Path $env:USERPROFILE 'go\bin\wails.exe'

if (-not (Test-Path $wails)) {
    throw "未找到 Wails CLI: $wails"
}

Write-Host "==> Building desktop host from $desktopDir"
Push-Location $desktopDir
try {
    & $wails build -clean
}
finally {
    Pop-Location
}

Write-Host "==> Output: $desktopDir\build\bin\fire-salamander-desktop.exe"
