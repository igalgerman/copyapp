<#
Builds docs\video\install.mp4 and docs\video\use.mp4 by driving the loopback-only app build (temp APPDATA, cleanup as in docs\e2e\run.ps1).
  pwsh -File docs\video\make\make.ps1 [-Only install|use|both] [-NoBuild] [-IllustrateNative] [-StartupPng <verified PNG>] [-ComposeOnly -WorkDir <recording directory>]
Needs: ffmpeg (winget install --id Gyan.FFmpeg -e), Node, Windows fonts. Overwrites the Windows clipboard (prior text is restored at the end).
Raw recordings go to %TEMP%\copyapp-video-<unique ID>\raw (not in git); final mp4s go to docs\video (not in git; attach them to the Release).
install.mp4 runs the real first run and self-installs into temporary folders.
By default, capdlg.ps1 and caposd.ps1 capture native startup/confirmation windows with ownership and visibility guards.
-StartupPng uses a separately verified native startup capture from the same EXE; startup is then answered automatically.
-IllustrateNative answers startup automatically and clearly labels startup/green-confirmation illustrations;
the use clip then demonstrates the actual PC-window Send clipboard button rather than claiming a recorded hotkey press.
Use this explicit fallback when native capture is unavailable. Actual app pages, pairing and transfers use the app with only its network binding restricted to this PC.
Install/shortcut folders, data and received files are temporary; browser opening and real autostart writes are disabled.
Public Wi-Fi names, usernames, IP text and displayed QR pixels are sanitized. Actual pairing decodes the real QR off-screen.
#>
param([ValidateSet('install','use','both')]$Only = 'both', [switch]$NoBuild, [switch]$ComposeOnly, [string]$WorkDir, [string]$StartupPng, [switch]$IllustrateNative, [ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version = '1.0.2')
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot\..\..\..").Path
$exe  = Join-Path $root 'output\test-bin\copyapp.exe'
if ($StartupPng) {
    $StartupPng = (Resolve-Path -LiteralPath $StartupPng).Path
    $signature = [IO.File]::ReadAllBytes($StartupPng)
    if ($signature.Length -lt 8 -or [Convert]::ToHexString($signature[0..7]) -ne '89504E470D0A1A0A') { throw 'StartupPng must be a verified native-window PNG capture' }
    Write-Host "Using separately verified native startup capture: $StartupPng"
}
$tempRoot = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
$work = if ($WorkDir) { [IO.Path]::GetFullPath($WorkDir) } else { Join-Path $env:TEMP ('copyapp-video-' + [guid]::NewGuid().ToString('N')) }
if (-not $work.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $work -Leaf) -notmatch '^copyapp-video-[a-f0-9]{32}$') { throw 'WorkDir must be a copyapp-video-<32 hex digits> directory within TEMP' }
if ($ComposeOnly -and -not $WorkDir) { throw 'ComposeOnly requires -WorkDir from the original recording' }
$node = Join-Path $env:TEMP 'copyapp-video-node'
$raw  = Join-Path $work 'raw'
function Remove-RecordingDirectory([string]$Path) {
    $target = [IO.Path]::GetFullPath($Path)
    if (-not $target.StartsWith($work.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw "Unsafe recording cleanup path: $target" }
    Remove-Item -LiteralPath $target -Recurse -Force -ErrorAction SilentlyContinue
}
function Stop-RecordingProcesses {
    # Self-install starts a child and exits. Only stop binaries belonging to this recording.
    foreach ($p in Get-Process copyapp -ErrorAction SilentlyContinue) {
        if (($proc -and $p.Id -eq $proc.Id -and $p.StartTime -eq $proc.StartTime) -or
            ($p.Path -and $p.Path.StartsWith($work.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase))) {
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        }
    }
}
Write-Host "Recording work directory: $work"
$ffmpeg = (Get-Command ffmpeg -ErrorAction SilentlyContinue).Source
if (-not $ffmpeg) { $ffmpeg = (Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Recurse -Filter ffmpeg.exe -ErrorAction SilentlyContinue | Select-Object -First 1).FullName }
if (-not $ffmpeg) { throw 'ffmpeg not found: winget install --id Gyan.FFmpeg -e' }
New-Item -ItemType Directory -Force $work, $raw | Out-Null
if (-not (Test-Path "$node\node_modules\playwright")) {
    New-Item -ItemType Directory -Force $node | Out-Null
    '{}' | Set-Content "$node\package.json"
    npm install --prefix $node --no-audit --no-fund playwright@1.49.1 jsqr@1.4.0 pngjs@7.0.0 | Out-Null
    if ($LASTEXITCODE) { throw 'recording dependencies installation failed' }
}
& node "$node\node_modules\playwright\cli.js" install chromium
if ($LASTEXITCODE) { throw 'Chromium installation failed' }
$env:NODE_PATH = "$node\node_modules"; $env:FFMPEG = $ffmpeg; $env:RAW_DIR = $raw
$env:OUT_DIR = Join-Path $root 'docs\video'; $env:WEB_DIR = Join-Path $root 'internal\server\web'; $env:COPYAPP_EXE = $exe
if (Get-Process copyapp -ErrorAction SilentlyContinue) { throw 'copyapp.exe is already running; quit it first' }
if (-not $NoBuild -and -not $ComposeOnly) {
    pwsh -NoProfile -File (Join-Path $root 'build.ps1') -LoopbackTest -Version $Version
    if ($LASTEXITCODE) { throw 'build failed' }
}
. (Join-Path $root 'docs\e2e\test-binary.ps1')
if (-not $ComposeOnly) { Assert-LoopbackTestBinary $exe }
$oldAppdata = $env:APPDATA
$testEnv = @('COPYAPP_NO_OSD','COPYAPP_PORTABLE','COPYAPP_NO_BROWSER','COPYAPP_SKIP_PROMPTS','COPYAPP_NO_AUTOSTART_WRITE','COPYAPP_RECEIVE_DIR','COPYAPP_DATA_DIR','COPYAPP_WIFI_NAME','COPYAPP_INSTALL_DIR','COPYAPP_DESKTOP_DIR','COPYAPP_STARTMENU_DIR','COPYAPP_SENDTO_DIR','COPYAPP_APPDATA','COPYAPP_LPORT','COPYAPP_PID','CAP_DIR','COPYAPP_ILLUSTRATE_NATIVE','COPYAPP_RECORD_VERSION')
$previousEnv = @{}; foreach ($key in $testEnv) { $previousEnv[$key] = [Environment]::GetEnvironmentVariable($key) }
$scen = if ($Only -eq 'both') { 'install','use' } else { @($Only) }
try {
    if (-not $ComposeOnly) {
        $env:COPYAPP_RECORD_VERSION = (Get-Item -LiteralPath $exe).VersionInfo.ProductVersion -replace '\.0$', ''
        if ($env:COPYAPP_RECORD_VERSION -ne $Version) { throw "Expected CopyApp $Version; rebuild the loopback test EXE with this version first" }
    }
    foreach ($s in $scen) {
        if (-not $ComposeOnly) {
            $appdata = Join-Path $work "appdata-$s"
            Remove-RecordingDirectory $appdata
            New-Item -ItemType Directory -Force $appdata | Out-Null
            $env:APPDATA = $appdata
            $dd = "$appdata\CopyApp"
            New-Item -ItemType Directory -Force $dd | Out-Null
            '{"language":"en"}' | Set-Content "$dd\settings.json"   # English windows and dialogs in the video
            $env:COPYAPP_NO_BROWSER = '1'; $env:COPYAPP_NO_AUTOSTART_WRITE = '1'
            $env:COPYAPP_NO_OSD = $null
            $env:COPYAPP_ILLUSTRATE_NATIVE = if ($IllustrateNative) { '1' } else { $null }
            $env:COPYAPP_WIFI_NAME = 'Home Wi-Fi'; $env:COPYAPP_RECEIVE_DIR = "$work\received-$s"; $env:COPYAPP_DATA_DIR = $dd
            $env:COPYAPP_INSTALL_DIR = "$work\installed"; $env:COPYAPP_DESKTOP_DIR = "$work\desktop"; $env:COPYAPP_STARTMENU_DIR = "$work\startmenu"; $env:COPYAPP_SENDTO_DIR = "$work\sendto"
            foreach ($d in 'installed','desktop','startmenu','sendto','cap','download') { Remove-RecordingDirectory "$work\$d" }
            $env:CAP_DIR = "$work\cap"; New-Item -ItemType Directory -Force "$work\cap" | Out-Null
            if ($s -eq 'install') {
                # REAL first run: the downloaded exe installs itself (temp install/shortcut dirs) and shows the real
                # "Start with Windows?" question; only that one window is captured, then answered Yes.
                New-Item -ItemType Directory -Force "$work\download" | Out-Null
                Copy-Item $exe "$work\download\copyapp.exe"
                $env:COPYAPP_PORTABLE = $null; $env:COPYAPP_SKIP_PROMPTS = $null
                if ($StartupPng -or $IllustrateNative) { $env:COPYAPP_SKIP_PROMPTS = 'yes' }
                $proc = Start-Process "$work\download\copyapp.exe" -WindowStyle Hidden -PassThru
                $env:APPDATA = $oldAppdata
                if ($StartupPng) { Copy-Item -LiteralPath $StartupPng -Destination "$work\cap\startup.png" }
                elseif (-not $IllustrateNative) {
                    & pwsh -NoProfile -File "$PSScriptRoot\capdlg.ps1" -Out "$work\cap\startup.png" -ExeRoot $work
                    if ($LASTEXITCODE) { throw 'startup question capture failed' }
                }
            } else {
                $env:COPYAPP_PORTABLE = '1'; $env:COPYAPP_SKIP_PROMPTS = 'no'
                $proc = Start-Process $exe -WindowStyle Hidden -PassThru
                $env:APPDATA = $oldAppdata
            }
            for ($i = 0; $i -lt 60 -and -not ((Test-Path "$dd\port.txt") -and (Test-Path "$dd\local-port.txt")); $i++) { Start-Sleep -Milliseconds 200 }
            if (-not (Test-Path "$dd\local-port.txt")) { throw 'CopyApp did not start within 12 seconds' }
            $env:COPYAPP_APPDATA = $appdata
            $env:COPYAPP_LPORT = (Get-Content "$dd\local-port.txt").Trim()
            $expectedExe = if ($s -eq 'install') { "$work\installed\copyapp.exe" } else { $exe }
            $running = @(Get-Process copyapp -ErrorAction SilentlyContinue | Where-Object Path -eq $expectedExe)
            if ($running.Count -ne 1) { throw 'Cannot identify the isolated CopyApp process' }
            Assert-LoopbackListeners $running[0].Id
            $env:COPYAPP_PID = [string]$running[0].Id
            Start-Sleep 2
            try { node "$PSScriptRoot\record.js" $s; if ($LASTEXITCODE) { throw "record $s failed" } }
            finally { Stop-RecordingProcesses; Start-Sleep 1 }
        }
        node "$PSScriptRoot\compose.js" $s; if ($LASTEXITCODE) { throw "compose $s failed" }
    }
}
finally {
    $env:APPDATA = $oldAppdata
    foreach ($key in $testEnv) { [Environment]::SetEnvironmentVariable($key, $previousEnv[$key]) }
    Stop-RecordingProcesses
    $resolvedWork = [IO.Path]::GetFullPath($work)
    if (-not $resolvedWork.StartsWith([IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe recording cleanup path' }
    Start-Sleep 1; foreach ($d in 'installed','desktop','startmenu','sendto','download') { Remove-RecordingDirectory (Join-Path $resolvedWork $d) }
    Get-ChildItem -LiteralPath $resolvedWork -Directory -Filter 'appdata-*' | ForEach-Object { Remove-RecordingDirectory $_.FullName }
}
