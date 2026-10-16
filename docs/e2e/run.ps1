<#
End-to-end check using the loopback-only app build (PowerShell 7+, Windows, Node for QR decoding).

  pwsh -File docs\e2e\run.ps1 [-NoBuild]

Runs the exe with APPDATA redirected to a temp dir, pairs like a phone (decodes
the pairing QR), sends text / a file / a PNG, pushes a file from the PC through
the real second-instance "Send to" path, and checks auth. It overwrites the
Windows clipboard. Data, receive folders and shortcuts are isolated in a
temporary test directory; prompts, browser opening and real autostart writes
are disabled. It also checks the real global hotkey and saved PC settings.
Refuses to run if copyapp.exe is already running.
Builds output\test-bin\copyapp.exe; release binaries are refused before launch.
Phone traffic stays on this PC, avoiding fresh LAN firewall prompts per test copy.
This covers the HTTP/exe behaviour; the 390x844 PWA UI was driven separately
with Playwright (HTTPS errors ignored, like a phone tab where the user tapped Advanced > Proceed).
#>
param([switch]$NoBuild, [switch]$LargeFiles, [switch]$Browser)
$ErrorActionPreference = 'Stop'
$root   = (Resolve-Path "$PSScriptRoot\..\..").Path
$exe    = Join-Path $root 'output\test-bin\copyapp.exe'
$tmp    = Join-Path $env:TEMP ('copyapp-e2e-' + [guid]::NewGuid().ToString('N'))
$appdata= Join-Path $tmp 'appdata'
$files  = Join-Path $tmp 'files'
$node   = Join-Path $env:TEMP 'copyapp-e2e-node'
. "$PSScriptRoot\test-binary.ps1"
$script:fails = 0
function Check($name, $ok, $extra = '') {
    if ($ok) { Write-Host "PASS  $name" } else { Write-Host "FAIL  $name $extra" -ForegroundColor Red; $script:fails++ }
}
if (Get-Process copyapp -ErrorAction SilentlyContinue) { throw 'copyapp.exe is already running; quit it first' }

$runKey  = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$sendTo  = Join-Path $tmp 'sendto\Phone (CopyApp).lnk'
$recv    = Join-Path $tmp 'received'
$hadRun  = $null -ne (Get-ItemProperty $runKey -Name CopyApp -ErrorAction SilentlyContinue)
$hadLnk  = Test-Path $sendTo
$hadRecv = Test-Path $recv
$oldAppdata = $env:APPDATA
$testEnv = @('COPYAPP_NO_OSD','COPYAPP_PORTABLE','COPYAPP_NO_BROWSER','COPYAPP_SKIP_PROMPTS','COPYAPP_NO_AUTOSTART_WRITE','COPYAPP_RECEIVE_DIR','COPYAPP_DATA_DIR','COPYAPP_WIFI_NAME','COPYAPP_INSTALL_DIR','COPYAPP_DESKTOP_DIR','COPYAPP_STARTMENU_DIR','COPYAPP_SENDTO_DIR')
$previousEnv = @{}; foreach ($key in $testEnv) { $previousEnv[$key] = [Environment]::GetEnvironmentVariable($key) }
$proc = $null
function Assert-TestPath([string]$path) {
    $full = [IO.Path]::GetFullPath($path)
    $base = [IO.Path]::GetFullPath($tmp).TrimEnd('\')
    if ($full -ne $base -and -not $full.StartsWith($base + '\', [StringComparison]::OrdinalIgnoreCase)) { throw "Unsafe e2e path: $full" }
    return $full
}
function Stop-TestApps {
    foreach ($p in @(Get-Process copyapp -ErrorAction SilentlyContinue)) {
        $path = $null; try { $path = $p.Path } catch {}
        if ($path -and ($path -eq $exe -or $path.StartsWith($tmp.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase))) {
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        }
    }
}
try {
    if (-not $NoBuild) {
        pwsh -NoProfile -File (Join-Path $root 'build.ps1') -LoopbackTest
        if ($LASTEXITCODE) { throw 'build failed' }
    }
    Assert-LoopbackTestBinary $exe
    foreach ($d in @($appdata, $files)) { $safe = Assert-TestPath $d; Remove-Item -LiteralPath $safe -Recurse -Force -ErrorAction SilentlyContinue }
    New-Item -ItemType Directory -Force $appdata, $files, $node | Out-Null
    if (-not (Test-Path "$node\node_modules\pngjs")) {
        '{}' | Set-Content "$node\package.json"
        npm install --prefix $node --no-audit --no-fund jsqr@1.4.0 pngjs@7.0.0 | Out-Null
        if ($LASTEXITCODE) { throw 'npm install failed' }
    }

    # fixtures
    'hello file e2e' | Set-Content "$files\note-e2e.txt" -NoNewline
    Add-Type -AssemblyName System.Drawing, System.Windows.Forms
    $bmp = New-Object System.Drawing.Bitmap 40, 30
    $g = [System.Drawing.Graphics]::FromImage($bmp); $g.Clear([System.Drawing.Color]::FromArgb(255, 200, 30, 60)); $g.Dispose()
    $bmp.Save("$files\pic-e2e.png", [System.Drawing.Imaging.ImageFormat]::Png); $bmp.Dispose()
    'pushed from PC e2e' | Set-Content "$files\fromPC.txt" -NoNewline

    # start the real exe
    $env:APPDATA = $appdata
    $env:COPYAPP_PORTABLE = '1'; $env:COPYAPP_NO_BROWSER = '1'; $env:COPYAPP_SKIP_PROMPTS = 'no'; $env:COPYAPP_NO_AUTOSTART_WRITE = '1'
    $env:COPYAPP_RECEIVE_DIR = $recv; $env:COPYAPP_DATA_DIR = "$appdata\CopyApp"; $env:COPYAPP_WIFI_NAME = 'Home Wi-Fi'
    $env:COPYAPP_INSTALL_DIR = "$tmp\installed"; $env:COPYAPP_DESKTOP_DIR = "$tmp\desktop"; $env:COPYAPP_STARTMENU_DIR = "$tmp\startmenu"; $env:COPYAPP_SENDTO_DIR = "$tmp\sendto"
    $proc = Start-Process $exe -WindowStyle Hidden -PassThru
    $dd = "$appdata\CopyApp"
    for ($i = 0; $i -lt 50 -and -not ((Test-Path "$dd\port.txt") -and (Test-Path "$dd\local-port.txt")); $i++) { Start-Sleep -Milliseconds 200 }
    $port = (Get-Content "$dd\port.txt").Trim(); $lport = (Get-Content "$dd\local-port.txt").Trim()
    $L = "http://127.0.0.1:$lport"
    Assert-LoopbackListeners $proc.Id
    $B = "https://localhost:$port"
    $opt = @{ SkipCertificateCheck = $true; SkipHttpErrorCheck = $true; TimeoutSec = 10 }
    Check 'ports written' ($port -and $lport)

    # phone-facing routes without a session
    foreach ($u in '/status', '/events', '/item/1') { Check "no cookie $u -> 401" ((Invoke-WebRequest "$B$u" @opt).StatusCode -eq 401) }
    Check 'no cookie POST /send -> 401' ((Invoke-WebRequest "$B/send" -Method POST -Body '{"text":"x"}' -ContentType application/json @opt).StatusCode -eq 401)
    $r = Invoke-WebRequest "$B/favicon.ico" @opt; Check 'favicon.ico 200 image/png' ($r.StatusCode -eq 200 -and "$($r.Headers.'Content-Type')" -eq 'image/png')
    $r = Invoke-WebRequest "$B/bricolage.woff2" @opt; Check 'woff2 served as font/woff2' ("$($r.Headers.'Content-Type')" -eq 'font/woff2')
    Check 'pairing page rejects a non-local Host' ((Invoke-WebRequest "$B/pairing" -Headers @{ Host = "192.0.2.1:$port" } @opt).StatusCode -eq 403)
    Check '/ca.crt available while pairing is open' ((Invoke-WebRequest "$B/ca.crt" @opt).StatusCode -eq 200)

    $ps = Invoke-RestMethod "$L/pairing/status" -Headers @{ 'X-CopyApp' = (Get-Content "$dd\local.key").Trim() }
    Check 'pairing window is 10 minutes' ($ps.open -and $ps.secondsLeft -gt 540 -and $ps.secondsLeft -le 600) "$($ps.secondsLeft)"
    # pair: decode the QR like a phone camera
    $lh = @{ 'X-CopyApp' = (Get-Content "$dd\local.key").Trim() }
    Check '/qr without the local key -> 403' ((Invoke-WebRequest "$L/qr" @opt).StatusCode -eq 403)
    Invoke-WebRequest "$L/qr" -Headers $lh -OutFile "$tmp\qr.png" | Out-Null
    $env:NODE_PATH = "$node\node_modules"
    $pairUrl = node "$PSScriptRoot\qr-decode.js" "$tmp\qr.png"
    Check 'QR decodes to a pair URL' ($pairUrl -match '^https://.+/pair\?t=[0-9a-f]{64}$') $pairUrl
    $pairUrl = $pairUrl -replace '^https://[^/]+', $B
    $r = Invoke-WebRequest $pairUrl -SessionVariable sess @opt
    $cookie = $sess.Cookies.GetAllCookies() | Where-Object Name -eq 'copyapp_session'
    Check 'pair link -> session cookie, redirected to PWA' ($r.StatusCode -eq 200 -and $cookie -and $cookie.Secure -and $cookie.HttpOnly)
    $hdr = @{ Cookie = "$($cookie.Name)=$($cookie.Value)" }
    $r = Invoke-WebRequest $pairUrl @opt; Check 'pair token is single use' ($r.StatusCode -eq 401)
    Check 'session: /status 200 paired' ((Invoke-RestMethod "$B/status" -Headers $hdr -SkipCertificateCheck).paired -eq $true)
    Check '/ca.crt closed without cookie after pairing' ((Invoke-WebRequest "$B/ca.crt" @opt).StatusCode -eq 404)
    Check '/ca.crt with session' ((Invoke-WebRequest "$B/ca.crt" -Headers $hdr @opt).StatusCode -eq 200)
    $st = Invoke-RestMethod "$B/status" -Headers $hdr -SkipCertificateCheck
    Check '/status defaults to 2 GiB' ($st.maxBytes -eq 2147483648)
    Check '/status defaults to clipboard' ($st.receiveMode -eq 'clipboard')
    Check '/status has a short certificate code' ("$($st.shortFingerprint)".Length -ge 6) "$($st.shortFingerprint)"
    $r = Invoke-WebRequest "$B/" -Headers $hdr @opt; Check 'PWA index served' ($r.StatusCode -eq 200 -and $r.Content -match 'CopyApp')
    # browser-tab first: no certificate step in the main flow, the app upgrade is optional (Settings)
    Check 'PWA has no certificate onboarding card' ($r.Content -notmatch 'setup-card|cert-teaser')
    Check 'PWA Settings has Verify connection + optional Make-it-an-app rows' ($r.Content -match 'id="verify-row"' -and $r.Content -match 'id="app-row"[^>]*data-open="sheet-cert"' -and $r.Content -match 'id="tip-card"')
    $pg = (Invoke-WebRequest "$L/pairing" -Headers $lh @opt).Content
    Check 'pairing page: exactly 2 steps' (([regex]::Matches($pg, '<li>')).Count -eq 2)
    $bi = $pg.IndexOf('<section class="before"'); $qi = $pg.IndexOf('<section class="qrbox"')
    Check 'pairing page: open "Before you scan" box above the QR with 3 points' ($bi -ge 0 -and $bi -lt $qi -and ([regex]::Matches($pg, '<li class="b">')).Count -eq 3 -and $pg.Substring($bi, $qi - $bi) -notmatch '<details')
    Check 'pairing page: collapsed "Can''t open the page?" troubleshooting' ($pg -match '<details id="trouble" class="trouble">')
    $ssid = 'Home Wi-Fi'
    if ($ssid) { Check "pairing page names this PC's Wi-Fi ($ssid)" ($pg -match 'data-k="pair.b1Named"' -and $pg.Contains('<bdi>' + [System.Net.WebUtility]::HtmlEncode($ssid) + '</bdi>')) }
    else { Check 'pairing page: generic same-Wi-Fi point (no Wi-Fi name known)' ($pg -match 'data-k="pair.b1"') }
    Check 'pairing page: short code in collapsed details, optional app section collapsed' ($pg -match '<details id="careful">' -and $pg -match '<details id="optional">' -and $pg -notmatch '<details open' -and $pg -match [regex]::Escape("$($st.shortFingerprint)"))

    # phone -> PC text (real Windows clipboard)
    Set-Clipboard 'SENTINEL-before'
    $text = "Hello from phone $([char]0x2713) e2e 123"
    $body = [Text.Encoding]::UTF8.GetBytes((@{ text = $text } | ConvertTo-Json))
    $r = Invoke-WebRequest "$B/send" -Method POST -Headers $hdr -ContentType 'application/json; charset=utf-8' -Body $body @opt
    Check 'send text -> 200' ($r.StatusCode -eq 200) $r.Content
    Start-Sleep -Milliseconds 500
    Check 'Windows clipboard has the text' ((Get-Clipboard) -eq $text) (Get-Clipboard)

    # Saved PC destination settings and explicit per-send override.
    $settings = @{ receiveMode = 'folder'; saveDir = $recv; maxBytes = 524288000 } | ConvertTo-Json
    $r = Invoke-WebRequest "$L/local/settings" -Headers $lh -Method POST -ContentType application/json -Body $settings @opt
    Check 'local settings changes receive mode and limit' ($r.StatusCode -eq 200)
    $st2 = Invoke-RestMethod "$B/status" -Headers $hdr -SkipCertificateCheck
    Check 'phone sees updated PC default and limit' ($st2.receiveMode -eq 'folder' -and $st2.maxBytes -eq 524288000)
    $r = Invoke-WebRequest "$B/send" -Headers $hdr -Method POST -ContentType application/json -Body '{"text":"saved as a file e2e","mode":"auto"}' @opt
    Check 'auto destination respects saved folder default' ($r.StatusCode -eq 200 -and ($r.Content | ConvertFrom-Json).where -eq 'folder')
    $r = Invoke-WebRequest "$B/send" -Headers $hdr -Method POST -ContentType application/json -Body '{"text":"explicit clipboard e2e","mode":"clipboard"}' @opt
    Check 'explicit clipboard overrides folder default' ($r.StatusCode -eq 200 -and ($r.Content | ConvertFrom-Json).where -eq 'clipboard')
    $null = Invoke-WebRequest "$L/local/settings" -Headers $lh -Method POST -ContentType application/json -Body '{"receiveMode":"clipboard","maxBytes":2147483648}' @opt

    # share-target fallback (POST /share, used when the service worker is not active)
    Check 'no cookie POST /share -> 401' ((Invoke-WebRequest "$B/share" -Method POST -Body '{"text":"x"}' -ContentType application/json @opt).StatusCode -eq 401)
    $r = Invoke-WebRequest "$B/share" -Method POST -Headers $hdr -ContentType 'application/json; charset=utf-8' -Body ([Text.Encoding]::UTF8.GetBytes('{"text":"shared via /share"}')) @opt
    Check 'POST /share text -> 200' ($r.StatusCode -eq 200) $r.Content
    Start-Sleep -Milliseconds 400
    Check 'clipboard has the /share text' ((Get-Clipboard) -eq 'shared via /share')
    $r = curl.exe -sk -o NUL -D - -X POST -H "Cookie: $($hdr.Cookie)" -H "Sec-Fetch-Mode: navigate" -F "file=@$files\fromPC.txt" "$B/share"
    Check 'POST /share navigation -> 303 to /?sent=' ("$r" -match ' 303 ' -and "$r" -match '(?i)location: /.sent=') "$r"
    Check '/share file saved' (Test-Path "$recv\fromPC.txt")

    # phone -> PC file
    $r = Invoke-WebRequest "$B/send" -Method POST -Headers $hdr -Form @{ file = Get-Item "$files\note-e2e.txt" } @opt
    Check 'send file -> 200' ($r.StatusCode -eq 200) $r.Content
    Check 'file saved in Downloads\CopyApp' ((Test-Path "$recv\note-e2e.txt") -and (Get-Content "$recv\note-e2e.txt" -Raw) -eq 'hello file e2e')
    $zone = Get-Content "$recv\note-e2e.txt" -Stream Zone.Identifier -ErrorAction SilentlyContinue
    Check 'received file has the Mark of the Web (ZoneId=3)' ("$zone" -match 'ZoneId=3') "$zone"
    '[InternetShortcut]' | Set-Content "$files\evil.lnk" -NoNewline
    $r = Invoke-WebRequest "$B/send" -Method POST -Headers $hdr -Form @{ file = Get-Item "$files\evil.lnk" } @opt
    Check 'received .lnk is renamed .lnk.blocked' ($r.StatusCode -eq 200 -and (Test-Path "$recv\evil.lnk.blocked") -and -not (Test-Path "$recv\evil.lnk")) $r.Content

    # phone -> PC image (goes to the clipboard as an image)
    Set-Clipboard 'SENTINEL-2'
    $r = Invoke-WebRequest "$B/send" -Method POST -Headers $hdr -Form ([ordered]@{ mode = 'clipboard'; file = Get-Item "$files\pic-e2e.png" }) @opt
    Check 'send png -> 200' ($r.StatusCode -eq 200) $r.Content
    Start-Sleep -Milliseconds 500
    # Clipboard.GetImage needs an STA thread; pwsh is MTA, so ask Windows PowerShell.
    $img = powershell.exe -NoProfile -STA -Command 'Add-Type -AssemblyName System.Windows.Forms; $i=[System.Windows.Forms.Clipboard]::GetImage(); if($i){"$($i.Width)x$($i.Height) R=$($i.GetPixel(0,0).R)"} else {"none; formats="+([System.Windows.Forms.Clipboard]::GetDataObject().GetFormats() -join ",")}'
    Check 'Windows clipboard has the 40x30 image' ("$img" -eq '40x30 R=200') $img

    # PC -> phone through the real second-instance path (Explorer "Send to")
    $p2 = Start-Process $exe -ArgumentList "`"$files\fromPC.txt`"" -WindowStyle Hidden -PassThru -Wait
    Check 'second instance exited 0' ($p2.ExitCode -eq 0)
    $ev = curl.exe -sk --max-time 3 -H "Cookie: $($hdr.Cookie)" "$B/events"
    Check 'pushed file is in the SSE stream' ("$ev" -match 'fromPC\.txt')
    $id = [regex]::Match("$ev", '"id":"([0-9a-f]+)"').Groups[1].Value
    $r = Invoke-WebRequest "$B/item/$id" -Headers $hdr @opt
    Check 'pushed item downloads with its content' ("$($r.Content)" -eq 'pushed from PC e2e')
    $range = Invoke-WebRequest "$B/item/$id" -Headers (@{ Cookie = $hdr.Cookie; Range = 'bytes=0-5' }) @opt
    Check 'disk-backed PC file supports partial download' ($range.StatusCode -eq 206 -and "$($range.Content)" -eq 'pushed')

    # Exercise the registered global hotkey rather than a substitute HTTP call.
    $pcState = Invoke-RestMethod "$L/local/state" -Headers $lh
    Check 'control reports an available global hotkey' ($pcState.hotkey -match '^Ctrl\+Alt\+(Shift\+)?C$') $pcState.hotkey
    Set-Clipboard 'hotkey-real-exe-e2e'
    Add-Type -Namespace CopyAppE2E -Name Keys -MemberDefinition '[System.Runtime.InteropServices.DllImport("user32.dll")] public static extern void keybd_event(byte key, byte scan, uint flags, System.UIntPtr extra);'
    $keys = @(0x11, 0x12); if ($pcState.hotkey -match 'Shift') { $keys += 0x10 }; $keys += 0x43
    foreach ($key in $keys) { [CopyAppE2E.Keys]::keybd_event($key, 0, 0, [UIntPtr]::Zero) }
    Start-Sleep -Milliseconds 120
    [array]::Reverse($keys)
    foreach ($key in $keys) { [CopyAppE2E.Keys]::keybd_event($key, 0, 2, [UIntPtr]::Zero) }
    Start-Sleep -Milliseconds 800
    $hotEvents = curl.exe -sk --max-time 3 -H "Cookie: $($hdr.Cookie)" "$B/events"
    Check 'global hotkey puts real clipboard text in phone inbox' ("$hotEvents" -match 'hotkey-real-exe-e2e')
    $log = Get-Content "$dd\copyapp.log" -Raw
    Check 'log: hotkey press, send request and publish recorded' ($log -match 'hotkey pressed: Ctrl\+Alt' -and $log -match 'clipboard send requested \(hotkey\)' -and $log -match 'published to phone: kind=text')
    # The same send from the CopyApp window button (same path as the tray item).
    Set-Clipboard 'window-button-e2e'
    $r = Invoke-RestMethod "$L/local/send-clipboard" -Method POST -Headers $lh
    Check 'window "send clipboard now" -> sent' ($r.result -eq 'sent') "$($r.result)"
    $ev2 = curl.exe -sk --max-time 3 -H "Cookie: $($hdr.Cookie)" "$B/events"
    Check 'window button puts clipboard text in phone inbox' ("$ev2" -match 'window-button-e2e')
    powershell.exe -NoProfile -STA -Command 'Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::Clear()'
    $r = Invoke-RestMethod "$L/local/send-clipboard" -Method POST -Headers $lh
    Check 'empty PC clipboard -> "empty"' ($r.result -eq 'empty') "$($r.result)"
    $expectedVersion = (Get-Item -LiteralPath $exe).VersionInfo.ProductVersion -replace '\.0$', ''
    Check 'control state reports built version and the pairing address' ($pcState.version -eq $expectedVersion -and $pcState.phone.pairedFrom) "$($pcState.version)"

    # loopback-only endpoints: 403 without the local key, 200 with it (header or cookie)
    foreach ($u in '/pairing/status') {
        Check "local $u without key -> 403" ((Invoke-WebRequest "$L$u" @opt).StatusCode -eq 403)
        Check "local $u wrong key -> 403" ((Invoke-WebRequest "$L$u" -Headers @{ 'X-CopyApp' = 'nope' } @opt).StatusCode -eq 403)
        Check "local $u with X-CopyApp -> 200" ((Invoke-WebRequest "$L$u" -Headers $lh @opt).StatusCode -eq 200)
    }
    Check 'local /qr after pairing with key -> 410 (authorized, window closed)' ((Invoke-WebRequest "$L/qr" -Headers $lh @opt).StatusCode -eq 410)
    Check 'pairing?k=wrong -> 403' ((Invoke-WebRequest "$L/pairing?k=nope" -MaximumRedirection 0 @opt).StatusCode -eq 403)
    $r = Invoke-WebRequest "$L/pairing?k=$($lh.'X-CopyApp')" -SessionVariable ls @opt
    $lc = $ls.Cookies.GetAllCookies() | Where-Object Name -eq 'copyapp_local'
    Check 'native bootstrap does not issue an ambient cookie' (-not $lc -and $r.StatusCode -eq 200)
    $match = [regex]::Match($r.Content, 'var token="([A-Za-z0-9_-]{43})"')
    $browserToken = $match.Groups[1].Value
    Check 'native bootstrap issues a separate browser token' ($match.Success -and $browserToken -ne $lh.'X-CopyApp')
    Check 'raw local key as cookie -> 403' ((Invoke-WebRequest "$L/pairing/status" -Headers @{ Cookie = "copyapp_local=$($lh.'X-CopyApp')" } @opt).StatusCode -eq 403)
    Check 'browser token as cookie -> 403' ((Invoke-WebRequest "$L/local/state" -Headers @{ Cookie = "copyapp_local=$browserToken" } @opt).StatusCode -eq 403)
    Check 'explicit browser header authorizes state' ((Invoke-WebRequest "$L/local/state" -Headers @{ 'X-CopyApp-Session' = $browserToken } @opt).StatusCode -eq 200)
    Check 'save folder on a network share -> 400' ((Invoke-WebRequest "$L/local/settings" -Headers $lh -Method POST -ContentType application/json -Body '{"saveDir":"\\\\server\\share"}' @opt).StatusCode -eq 400)
    $startup = Join-Path $oldAppdata 'Microsoft\Windows\Start Menu\Programs\Startup'
    Check 'save folder in Startup -> 400' ((Invoke-WebRequest "$L/local/settings" -Headers $lh -Method POST -ContentType application/json -Body (@{ saveDir = $startup } | ConvertTo-Json) @opt).StatusCode -eq 400)
    Check 'cookie-only control request is rejected' ((Invoke-WebRequest "$L/pairing/status" -WebSession $ls @opt).StatusCode -eq 403)
    Check 'pairing/open without key -> 403' ((Invoke-WebRequest "$L/pairing/open" -Method POST @opt).StatusCode -eq 403)
    Check 'send-file without X-CopyApp -> 403' ((Invoke-WebRequest "$L/local/send-file?path=x" -Method POST @opt).StatusCode -eq 403)
    Check 'send-file with a wrong key -> 403' ((Invoke-WebRequest "$L/local/send-file?path=x" -Method POST -Headers @{ 'X-CopyApp' = '1' } @opt).StatusCode -eq 403)
    Check 'send-file with Origin -> 403' ((Invoke-WebRequest "$L/local/send-file?path=x" -Method POST -Headers @{ 'X-CopyApp' = $lh.'X-CopyApp'; Origin = 'http://evil' } @opt).StatusCode -eq 403)
    Check 'pairing/open cross-origin -> 403' ((Invoke-WebRequest "$L/pairing/open" -Method POST -Headers @{ Origin = 'http://evil' } @opt).StatusCode -eq 403)

    if ($LargeFiles) {
        & (Join-Path $PSScriptRoot 'large-files.ps1') -BaseURL $B -LocalURL $L -LocalKey $lh.'X-CopyApp' -Cookie $hdr.Cookie -WorkDir $tmp -ReceiveDir $recv
        Check 'real 2 GiB streaming, limit, cancellation and resume checks' $true
    }
    if ($Browser) {
        $env:NODE_PATH = Join-Path $env:TEMP 'copyapp-video/node_modules'
        node (Join-Path $PSScriptRoot 'browser.js')
        Check 'real browser integration' ($LASTEXITCODE -eq 0)
        # The browser integration intentionally unpairs; obtain a fresh phone session for restart checks.
        $null = Invoke-WebRequest "$L/pairing/open" -Method POST -Headers @{ 'X-CopyApp'=$lh.'X-CopyApp'; Origin=$L } @opt
        Invoke-WebRequest "$L/qr" -Headers $lh -OutFile "$tmp/qr.png" | Out-Null
        $env:NODE_PATH = "$node/node_modules"
        $pairUrl = node "$PSScriptRoot/qr-decode.js" "$tmp/qr.png"
        $pairUrl = $pairUrl -replace '^https://[^/]+', $B
        $r = Invoke-WebRequest $pairUrl -SessionVariable sess @opt
        $cookie = $sess.Cookies.GetAllCookies() | Where-Object Name -eq 'copyapp_session'
        $hdr = @{ Cookie = "$($cookie.Name)=$($cookie.Value)" }
    }

    # session survives a restart; unpair revokes
    $null = Invoke-WebRequest "$L/local/settings" -Headers $lh -Method POST -ContentType application/json -Body '{"receiveMode":"folder","maxBytes":524288000,"language":"ru"}' @opt
    Stop-Process $proc -Force; Start-Sleep 1
    $proc = Start-Process $exe -WindowStyle Hidden -PassThru
    Start-Sleep 3
    $L = "http://127.0.0.1:$((Get-Content "$dd\local-port.txt").Trim())"
    Check 'session survives restart' ((Invoke-WebRequest "$B/status" -Headers $hdr @opt).StatusCode -eq 200)
    $state = Invoke-RestMethod "$L/local/state" -Headers @{ 'X-CopyApp' = (Get-Content "$dd\local.key").Trim() }
    Check 'settings persist across restart' ($state.settings.maxBytes -eq 524288000 -and $state.settings.receiveMode -eq 'folder' -and $state.settings.language -eq 'ru' -and $state.settings.saveDir -eq $recv) ($state.settings | ConvertTo-Json -Compress)
    $log = Get-Content "$dd\copyapp.log" -Raw
    $secrets = @('hotkey-real-exe-e2e', 'window-button-e2e', $text, 'explicit clipboard e2e', $lh.'X-CopyApp', $cookie.Value, ($pairUrl -replace '^.*t=', ''))
    Check 'log never contains clipboard text, the local key, session or pairing token' (-not ($secrets | Where-Object { $_ -and $log.Contains($_) }))
    Check 'log names the version' ($log -match 'CopyApp \d+\.\d+\.\d+ starting')
    Check 'unpair -> 204' ((Invoke-WebRequest "$B/unpair" -Method POST -Headers $hdr @opt).StatusCode -eq 204)
    Check 'after unpair /status -> 401' ((Invoke-WebRequest "$B/status" -Headers $hdr @opt).StatusCode -eq 401)

    # First run like a user: double-click the download -> it installs itself
    # (temp install/shortcut folders), answers the startup question (yes) and
    # runs the installed copy; the Desktop shortcut target opens the running
    # copy; uninstall removes only what install created.
    Stop-Process $proc -Force; Start-Sleep 1
    Remove-Item Env:COPYAPP_PORTABLE
    $env:COPYAPP_SKIP_PROMPTS = 'yes'
    $env:COPYAPP_DATA_DIR = "$tmp\appdata2\CopyApp"; $dd2 = $env:COPYAPP_DATA_DIR
    $download = "$tmp\download\copyapp.exe"; New-Item -ItemType Directory -Force (Split-Path $download) | Out-Null; Copy-Item $exe $download
    # Not -Wait: that would also wait for the installed copy it starts.
    $p1 = Start-Process $download -WindowStyle Hidden -PassThru; $null = $p1.WaitForExit(60000)
    Check 'downloaded exe hands over to the installed copy and exits 0 within 5 s' ($p1.HasExited -and $p1.ExitCode -eq 0 -and ($p1.ExitTime - $p1.StartTime).TotalSeconds -lt 5) "$(($p1.ExitTime - $p1.StartTime).TotalSeconds) s"
    $inst = "$tmp\installed\copyapp.exe"
    Check 'installed copy in the install folder' ((Test-Path $inst) -and (Get-FileHash $inst).Hash -eq (Get-FileHash $download).Hash)
    for ($i = 0; $i -lt 50 -and -not (Test-Path "$tmp\sendto\Phone (CopyApp).lnk"); $i++) { Start-Sleep -Milliseconds 200 }
    Check 'Desktop, Start menu and Send to shortcuts created' ((Test-Path "$tmp\desktop\CopyApp.lnk") -and (Test-Path "$tmp\startmenu\CopyApp.lnk") -and (Test-Path "$tmp\sendto\Phone (CopyApp).lnk"))
    $sh = (New-Object -ComObject WScript.Shell).CreateShortcut("$tmp\desktop\CopyApp.lnk")
    Check 'Desktop shortcut opens the installed exe with --open' ($sh.TargetPath -eq $inst -and $sh.Arguments -eq '--open') "$($sh.TargetPath) $($sh.Arguments)"
    for ($i = 0; $i -lt 50 -and -not (Test-Path "$dd2\local-port.txt"); $i++) { Start-Sleep -Milliseconds 200 }
    Start-Sleep 1
    $running = Get-Process copyapp -ErrorAction SilentlyContinue
    Check 'exactly one copy runs, the installed one' (@($running).Count -eq 1 -and $running.Path -eq $inst) "$($running.Path)"
    Assert-LoopbackListeners $running.Id
    $lh2 = @{ 'X-CopyApp' = (Get-Content "$dd2\local.key").Trim() }; $L2 = "http://127.0.0.1:$((Get-Content "$dd2\local-port.txt").Trim())"
    $st2 = Invoke-RestMethod "$L2/local/state" -Headers $lh2
    Check 'first run: startup answered, pairing window open' ((Get-Content "$dd2\settings.json" -Raw | ConvertFrom-Json).autostartAsked -and $st2.autostart -and $st2.pairing.open)
    $p2 = Start-Process $inst -ArgumentList '--open' -WindowStyle Hidden -PassThru; $null = $p2.WaitForExit(30000)
    Check 'Desktop shortcut while running: forwards to the running copy and exits' ($p2.ExitCode -eq 0 -and @(Get-Process copyapp).Count -eq 1)
    New-Item -ItemType Directory -Force "$tmp\received" | Out-Null; 'keep me' | Set-Content "$tmp\received\mine.txt"
    $r = Invoke-WebRequest "$L2/local/uninstall" -Method POST -Headers $lh2 @opt
    Check 'uninstall accepted' ($r.StatusCode -eq 202)
    for ($i = 0; $i -lt 60 -and ((Test-Path $inst) -or (Get-Process copyapp -ErrorAction SilentlyContinue)); $i++) { Start-Sleep -Milliseconds 500 }
    Check 'uninstall: app quit, exe and install folder removed' (-not (Get-Process copyapp -ErrorAction SilentlyContinue) -and -not (Test-Path "$tmp\installed"))
    Check 'uninstall: shortcuts removed' (-not (Test-Path "$tmp\desktop\CopyApp.lnk") -and -not (Test-Path "$tmp\startmenu\CopyApp.lnk") -and -not (Test-Path "$tmp\sendto\Phone (CopyApp).lnk"))
    Check 'uninstall: CopyApp data removed (answered yes)' (-not (Test-Path "$dd2\local.key") -and -not (Test-Path "$dd2\settings.json"))
    Check 'uninstall: received and unrelated files kept' ((Test-Path "$tmp\received\mine.txt") -and (Test-Path "$tmp\download\copyapp.exe"))
}
finally {
    $env:APPDATA = $oldAppdata
    foreach ($key in $testEnv) { [Environment]::SetEnvironmentVariable($key, $previousEnv[$key]) }
    Stop-TestApps
    if (-not $hadLnk) { Remove-Item $sendTo -Force -ErrorAction SilentlyContinue }
    foreach ($f in 'note-e2e.txt', 'pic-e2e.png', 'fromPC.txt') { Remove-Item (Join-Path $recv $f) -Force -ErrorAction SilentlyContinue }
    if (-not $hadRecv -and (Test-Path $recv) -and -not (Get-ChildItem $recv)) { Remove-Item $recv -Force }
    $resolvedTmp = [IO.Path]::GetFullPath($tmp)
    if (-not $resolvedTmp.StartsWith([IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe e2e cleanup path' }
    Remove-Item -LiteralPath $resolvedTmp -Recurse -Force -ErrorAction SilentlyContinue
}
if ($script:fails) { Write-Host "$($script:fails) check(s) failed" -ForegroundColor Red; exit 1 }
Write-Host 'All checks passed' -ForegroundColor Green
