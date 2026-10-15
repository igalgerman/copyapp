<#
Builds docs\video\install.mp4 and docs\video\use.mp4 by driving the REAL dist\copyapp.exe (temp APPDATA, cleanup as in docs\e2e\run.ps1).
  pwsh -File docs\video\make\make.ps1 [-Only install|use|both] [-NoBuild] [-ComposeOnly]
Needs: ffmpeg (winget install --id Gyan.FFmpeg -e), Node, Windows fonts. Overwrites the Windows clipboard (restored at the end).
Raw recordings go to %TEMP%\copyapp-video\raw (not in git); final mp4s go to docs\video (not in git; attach them to the Release).
The exe opens the default browser to the pairing page on start (cannot be disabled); close that tab.
#>
param([ValidateSet('install','use','both')]$Only = 'both', [switch]$NoBuild, [switch]$ComposeOnly)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot\..\..\..").Path
$exe  = Join-Path $root 'dist\copyapp.exe'
$work = Join-Path $env:TEMP 'copyapp-video'
$raw  = Join-Path $work 'raw'
$ffmpeg = (Get-Command ffmpeg -ErrorAction SilentlyContinue).Source
if (-not $ffmpeg) { $ffmpeg = (Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Recurse -Filter ffmpeg.exe -ErrorAction SilentlyContinue | Select-Object -First 1).FullName }
if (-not $ffmpeg) { throw 'ffmpeg not found: winget install --id Gyan.FFmpeg -e' }
New-Item -ItemType Directory -Force $work, $raw | Out-Null
if (-not (Test-Path "$work\node_modules\playwright")) {
    '{}' | Set-Content "$work\package.json"
    npm install --prefix $work --no-audit --no-fund playwright@1.49.1 jsqr@1.4.0 pngjs@7.0.0 | Out-Null
    Push-Location $work; npx playwright install chromium | Out-Null; Pop-Location
}
$env:NODE_PATH = "$work\node_modules"; $env:FFMPEG = $ffmpeg; $env:RAW_DIR = $raw
$env:OUT_DIR = Join-Path $root 'docs\video'; $env:WEB_DIR = Join-Path $root 'internal\server\web'; $env:COPYAPP_EXE = $exe
if (Get-Process copyapp -ErrorAction SilentlyContinue) { throw 'copyapp.exe is already running; quit it first' }
if (-not $NoBuild -and -not $ComposeOnly) {
    $env:PATH = "C:\Program Files\Go\bin;$env:PATH"
    Push-Location $root; go build -ldflags "-H=windowsgui -s -w" -o $exe ./cmd/copyapp; Pop-Location
    if ($LASTEXITCODE) { throw 'build failed' }
}
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$sendTo = Join-Path ([Environment]::GetFolderPath('SendTo')) 'Phone (CopyApp).lnk'
$hadRun = $null -ne (Get-ItemProperty $runKey -Name CopyApp -ErrorAction SilentlyContinue)
$hadLnk = Test-Path $sendTo
$oldAppdata = $env:APPDATA
$scen = if ($Only -eq 'both') { 'install','use' } else { @($Only) }
try {
    foreach ($s in $scen) {
        if (-not $ComposeOnly) {
            $appdata = Join-Path $work "appdata-$s"
            Remove-Item $appdata -Recurse -Force -ErrorAction SilentlyContinue
            New-Item -ItemType Directory -Force $appdata | Out-Null
            $env:APPDATA = $appdata
            $proc = Start-Process $exe -PassThru
            $env:APPDATA = $oldAppdata
            $dd = "$appdata\CopyApp"
            for ($i = 0; $i -lt 60 -and -not ((Test-Path "$dd\port.txt") -and (Test-Path "$dd\local-port.txt")); $i++) { Start-Sleep -Milliseconds 200 }
            $env:COPYAPP_APPDATA = $appdata
            $env:COPYAPP_LPORT = (Get-Content "$dd\local-port.txt").Trim()
            Start-Sleep 2
            try { node "$PSScriptRoot\record.js" $s; if ($LASTEXITCODE) { throw "record $s failed" } }
            finally { Get-Process copyapp -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1 }
        }
        node "$PSScriptRoot\compose.js" $s; if ($LASTEXITCODE) { throw "compose $s failed" }
    }
}
finally {
    $env:APPDATA = $oldAppdata
    Get-Process copyapp -ErrorAction SilentlyContinue | Stop-Process -Force
    if (-not $hadRun) { Remove-ItemProperty $runKey -Name CopyApp -ErrorAction SilentlyContinue }
    if (-not $hadLnk) { Remove-Item $sendTo -Force -ErrorAction SilentlyContinue }
    Remove-Item (Join-Path $work 'appdata-*') -Recurse -Force -ErrorAction SilentlyContinue
}
