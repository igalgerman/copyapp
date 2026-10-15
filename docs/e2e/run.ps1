<#
End-to-end check of the REAL dist\copyapp.exe (PowerShell 7+, Windows, Node for QR decoding).

  pwsh -File docs\e2e\run.ps1 [-NoBuild]

Runs the exe with APPDATA redirected to a temp dir, pairs like a phone (decodes
the pairing QR), sends text / a file / a PNG, pushes a file from the PC through
the real second-instance "Send to" path, and checks auth. It overwrites the
Windows clipboard. The exe cannot redirect three things, which the script
removes again at the end: HKCU Run value 'CopyApp', the SendTo shortcut, and
files in Downloads\CopyApp (only those this test created).
Refuses to run if copyapp.exe is already running.
This covers the HTTP/exe behaviour; the 390x844 PWA UI was driven separately
with Playwright (HTTPS errors ignored).
#>
param([switch]$NoBuild)
$ErrorActionPreference = 'Stop'
$root   = (Resolve-Path "$PSScriptRoot\..\..").Path
$exe    = Join-Path $root 'dist\copyapp.exe'
$tmp    = Join-Path $env:TEMP 'copyapp-e2e'
$appdata= Join-Path $tmp 'appdata'
$files  = Join-Path $tmp 'files'
$node   = Join-Path $tmp 'node'
$script:fails = 0
function Check($name, $ok, $extra = '') {
    if ($ok) { Write-Host "PASS  $name" } else { Write-Host "FAIL  $name $extra" -ForegroundColor Red; $script:fails++ }
}
if (Get-Process copyapp -ErrorAction SilentlyContinue) { throw 'copyapp.exe is already running; quit it first' }

$runKey  = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$sendTo  = Join-Path ([Environment]::GetFolderPath('SendTo')) 'Phone (CopyApp).lnk'
$recv    = Join-Path (New-Object -ComObject Shell.Application).NameSpace('shell:Downloads').Self.Path 'CopyApp'
$hadRun  = $null -ne (Get-ItemProperty $runKey -Name CopyApp -ErrorAction SilentlyContinue)
$hadLnk  = Test-Path $sendTo
$hadRecv = Test-Path $recv
$oldAppdata = $env:APPDATA
$proc = $null
try {
    if (-not $NoBuild) {
        $env:PATH = "C:\Program Files\Go\bin;$env:PATH"
        Push-Location $root; go build -ldflags "-H=windowsgui -s -w" -o $exe ./cmd/copyapp; Pop-Location
        if ($LASTEXITCODE) { throw 'build failed' }
    }
    Remove-Item $appdata, $files -Recurse -Force -ErrorAction SilentlyContinue
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
    $proc = Start-Process $exe -PassThru
    $dd = "$appdata\CopyApp"
    for ($i = 0; $i -lt 50 -and -not ((Test-Path "$dd\port.txt") -and (Test-Path "$dd\local-port.txt")); $i++) { Start-Sleep -Milliseconds 200 }
    $port = (Get-Content "$dd\port.txt").Trim(); $lport = (Get-Content "$dd\local-port.txt").Trim()
    $L = "http://127.0.0.1:$lport"
    $ip = ([System.Net.NetworkInformation.NetworkInterface]::GetAllNetworkInterfaces() | Where-Object OperationalStatus -eq Up |
        ForEach-Object { $_.GetIPProperties().UnicastAddresses } |
        Where-Object { $_.Address.AddressFamily -eq 'InterNetwork' -and $_.Address.ToString() -match '^(10\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.)' } |
        Select-Object -First 1).Address.ToString()
    $B = "https://${ip}:$port"
    $opt = @{ SkipCertificateCheck = $true; SkipHttpErrorCheck = $true; TimeoutSec = 10 }
    Check 'ports written' ($port -and $lport)

    # phone-facing routes without a session
    foreach ($u in '/status', '/events', '/item/1') { Check "no cookie $u -> 401" ((Invoke-WebRequest "$B$u" @opt).StatusCode -eq 401) }
    Check 'no cookie POST /send -> 401' ((Invoke-WebRequest "$B/send" -Method POST -Body '{"text":"x"}' -ContentType application/json @opt).StatusCode -eq 401)
    $r = Invoke-WebRequest "$B/favicon.ico" @opt; Check 'favicon.ico 200 image/png' ($r.StatusCode -eq 200 -and "$($r.Headers.'Content-Type')" -eq 'image/png')
    $r = Invoke-WebRequest "$B/bricolage.woff2" @opt; Check 'woff2 served as font/woff2' ("$($r.Headers.'Content-Type')" -eq 'font/woff2')
    Check 'pairing page is not reachable from the LAN address' ((Invoke-WebRequest "$B/pairing" @opt).StatusCode -eq 403)
    Check '/ca.crt available while pairing is open' ((Invoke-WebRequest "$B/ca.crt" @opt).StatusCode -eq 200)

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
    Check '/status has a short certificate code' ("$($st.shortFingerprint)".Length -ge 6) "$($st.shortFingerprint)"
    $r = Invoke-WebRequest "$B/" -Headers $hdr @opt; Check 'PWA index served' ($r.StatusCode -eq 200 -and $r.Content -match 'CopyApp')

    # phone -> PC text (real Windows clipboard)
    Set-Clipboard 'SENTINEL-before'
    $text = "Hello from phone $([char]0x2713) e2e 123"
    $body = [Text.Encoding]::UTF8.GetBytes((@{ text = $text } | ConvertTo-Json))
    $r = Invoke-WebRequest "$B/send" -Method POST -Headers $hdr -ContentType 'application/json; charset=utf-8' -Body $body @opt
    Check 'send text -> 200' ($r.StatusCode -eq 200) $r.Content
    Start-Sleep -Milliseconds 500
    Check 'Windows clipboard has the text' ((Get-Clipboard) -eq $text) (Get-Clipboard)

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

    # phone -> PC image (goes to the clipboard as an image)
    Set-Clipboard 'SENTINEL-2'
    $r = Invoke-WebRequest "$B/send" -Method POST -Headers $hdr -Form ([ordered]@{ mode = 'clipboard'; file = Get-Item "$files\pic-e2e.png" }) @opt
    Check 'send png -> 200' ($r.StatusCode -eq 200) $r.Content
    Start-Sleep -Milliseconds 500
    # Clipboard.GetImage needs an STA thread; pwsh is MTA, so ask Windows PowerShell.
    $img = powershell.exe -NoProfile -STA -Command 'Add-Type -AssemblyName System.Windows.Forms; $i=[System.Windows.Forms.Clipboard]::GetImage(); if($i){"$($i.Width)x$($i.Height) R=$($i.GetPixel(0,0).R)"} else {"none; formats="+([System.Windows.Forms.Clipboard]::GetDataObject().GetFormats() -join ",")}'
    Check 'Windows clipboard has the 40x30 image' ("$img" -eq '40x30 R=200') $img

    # PC -> phone through the real second-instance path (Explorer "Send to")
    $p2 = Start-Process $exe -ArgumentList "`"$files\fromPC.txt`"" -PassThru -Wait
    Check 'second instance exited 0' ($p2.ExitCode -eq 0)
    $ev = curl.exe -sk --max-time 3 -H "Cookie: $($hdr.Cookie)" "$B/events"
    Check 'pushed file is in the SSE stream' ("$ev" -match 'fromPC\.txt')
    $id = [regex]::Match("$ev", '"id":"([0-9a-f]+)"').Groups[1].Value
    $r = Invoke-WebRequest "$B/item/$id" -Headers $hdr @opt
    Check 'pushed item downloads with its content' ("$($r.Content)" -eq 'pushed from PC e2e')

    # loopback-only endpoints: 403 without the local key, 200 with it (header or cookie)
    foreach ($u in '/pairing', '/pairing/status') {
        Check "local $u without key -> 403" ((Invoke-WebRequest "$L$u" @opt).StatusCode -eq 403)
        Check "local $u wrong key -> 403" ((Invoke-WebRequest "$L$u" -Headers @{ 'X-CopyApp' = 'nope' } @opt).StatusCode -eq 403)
        Check "local $u with X-CopyApp -> 200" ((Invoke-WebRequest "$L$u" -Headers $lh @opt).StatusCode -eq 200)
    }
    Check 'local /qr after pairing with key -> 410 (authorized, window closed)' ((Invoke-WebRequest "$L/qr" -Headers $lh @opt).StatusCode -eq 410)
    Check 'pairing?k=wrong -> 403' ((Invoke-WebRequest "$L/pairing?k=nope" -MaximumRedirection 0 @opt).StatusCode -eq 403)
    $r = Invoke-WebRequest "$L/pairing?k=$($lh.'X-CopyApp')" -SessionVariable ls @opt
    $lc = $ls.Cookies.GetAllCookies() | Where-Object Name -eq 'copyapp_local'
    Check 'pairing?k=KEY sets copyapp_local cookie, redirects to /pairing' ($r.StatusCode -eq 200 -and $lc -and $lc.HttpOnly -and $r.BaseResponse.RequestMessage.RequestUri.AbsolutePath -eq '/pairing')
    Check 'local cookie alone opens /pairing/status' ((Invoke-WebRequest "$L/pairing/status" -WebSession $ls @opt).StatusCode -eq 200)
    Check 'pairing/open without key -> 403' ((Invoke-WebRequest "$L/pairing/open" -Method POST @opt).StatusCode -eq 403)
    Check 'send-file without X-CopyApp -> 403' ((Invoke-WebRequest "$L/local/send-file?path=x" -Method POST @opt).StatusCode -eq 403)
    Check 'send-file with a wrong key -> 403' ((Invoke-WebRequest "$L/local/send-file?path=x" -Method POST -Headers @{ 'X-CopyApp' = '1' } @opt).StatusCode -eq 403)
    Check 'send-file with Origin -> 403' ((Invoke-WebRequest "$L/local/send-file?path=x" -Method POST -Headers @{ 'X-CopyApp' = $lh.'X-CopyApp'; Origin = 'http://evil' } @opt).StatusCode -eq 403)
    Check 'pairing/open cross-origin -> 403' ((Invoke-WebRequest "$L/pairing/open" -Method POST -Headers @{ Origin = 'http://evil' } @opt).StatusCode -eq 403)

    # session survives a restart; unpair revokes
    Stop-Process $proc -Force; Start-Sleep 1
    $proc = Start-Process $exe -PassThru
    Start-Sleep 3
    Check 'session survives restart' ((Invoke-WebRequest "$B/status" -Headers $hdr @opt).StatusCode -eq 200)
    Check 'unpair -> 204' ((Invoke-WebRequest "$B/unpair" -Method POST -Headers $hdr @opt).StatusCode -eq 204)
    Check 'after unpair /status -> 401' ((Invoke-WebRequest "$B/status" -Headers $hdr @opt).StatusCode -eq 401)
}
finally {
    $env:APPDATA = $oldAppdata
    Get-Process copyapp -ErrorAction SilentlyContinue | Stop-Process -Force
    if (-not $hadRun) { Remove-ItemProperty $runKey -Name CopyApp -ErrorAction SilentlyContinue }
    if (-not $hadLnk) { Remove-Item $sendTo -Force -ErrorAction SilentlyContinue }
    foreach ($f in 'note-e2e.txt', 'pic-e2e.png', 'fromPC.txt') { Remove-Item (Join-Path $recv $f) -Force -ErrorAction SilentlyContinue }
    if (-not $hadRecv -and (Test-Path $recv) -and -not (Get-ChildItem $recv)) { Remove-Item $recv -Force }
    Remove-Item $appdata, $files -Recurse -Force -ErrorAction SilentlyContinue
}
if ($script:fails) { Write-Host "$($script:fails) check(s) failed" -ForegroundColor Red; exit 1 }
Write-Host 'All checks passed' -ForegroundColor Green
