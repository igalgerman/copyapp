<#
Native desktop integration checks using a loopback-only test build. Run after run.ps1:
  pwsh -NoProfile -File docs/e2e/desktop.ps1 [-Exe output/test-bin/copyapp.exe] [-BaselineExe previous-loopback-test.exe] [-TimeoutSec 60]
Requires an interactive Windows desktop and the QR decoder modules installed by run.ps1.
All application folders and shortcuts are isolated; autostart writes and browser opening are disabled.
#>
param([string]$Exe, [string]$BaselineExe, [ValidateRange(10,180)][int]$TimeoutSec = 60, [string]$NodeModules)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot\..\..").Path
if (-not $Exe) { $Exe = Join-Path $root 'output\test-bin\copyapp.exe' }
$Exe = (Resolve-Path -LiteralPath $Exe).Path
if (-not $BaselineExe) { $BaselineExe = $Exe }
$BaselineExe = (Resolve-Path -LiteralPath $BaselineExe).Path
. "$PSScriptRoot\test-binary.ps1"
Assert-LoopbackTestBinary $Exe
Assert-LoopbackTestBinary $BaselineExe
if (-not $NodeModules) { $NodeModules = Join-Path $env:TEMP 'copyapp-e2e-node\node_modules' }
if (-not (Test-Path (Join-Path $NodeModules 'jsqr'))) { throw 'Run docs/e2e/run.ps1 first, or supply -NodeModules containing jsqr and pngjs' }
if (Get-Process copyapp -ErrorAction SilentlyContinue) { throw 'Quit the existing CopyApp before running desktop integration checks' }
$tmp = Join-Path $env:TEMP ('copyapp-desktop-' + [guid]::NewGuid().ToString('N'))
$tempPrefix = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
if (-not [IO.Path]::GetFullPath($tmp).StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe desktop test root' }
$ownedPrefix = [IO.Path]::GetFullPath($tmp).TrimEnd('\') + '\'
$names = @('APPDATA','NODE_PATH','COPYAPP_PORTABLE','COPYAPP_NO_BROWSER','COPYAPP_SKIP_PROMPTS','COPYAPP_NO_AUTOSTART_WRITE','COPYAPP_NO_OSD','COPYAPP_WIFI_NAME','COPYAPP_DATA_DIR','COPYAPP_RECEIVE_DIR','COPYAPP_INSTALL_DIR','COPYAPP_DESKTOP_DIR','COPYAPP_STARTMENU_DIR','COPYAPP_SENDTO_DIR')
$before = @{}; foreach ($name in $names) { $before[$name] = [Environment]::GetEnvironmentVariable($name) }
function Assert($name, [bool]$condition) { if (-not $condition) { throw "FAIL  $name" }; Write-Host "PASS  $name" }
function Wait-Until([string]$Name, [scriptblock]$Condition) {
    $until = [DateTime]::UtcNow.AddSeconds($TimeoutSec)
    do { if (& $Condition) { Write-Host "PASS  $Name"; return }; Start-Sleep -Milliseconds 150 } while ([DateTime]::UtcNow -lt $until)
    throw "FAIL  $Name (timeout)"
}
function Owned-Processes { @(Get-Process copyapp -ErrorAction SilentlyContinue | Where-Object { $_.Path -and $_.Path.StartsWith($ownedPrefix, [StringComparison]::OrdinalIgnoreCase) }) }
function Stop-Owned {
    foreach ($p in Owned-Processes) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
function Configure([string]$Phase) {
    $env:APPDATA = "$tmp\$Phase\appdata"
    $env:COPYAPP_DATA_DIR = "$env:APPDATA\CopyApp"
    $env:COPYAPP_RECEIVE_DIR = "$tmp\$Phase\received"
    $env:COPYAPP_INSTALL_DIR = "$tmp\$Phase\installed"
    $env:COPYAPP_DESKTOP_DIR = "$tmp\$Phase\desktop"
    $env:COPYAPP_STARTMENU_DIR = "$tmp\$Phase\startmenu"
    $env:COPYAPP_SENDTO_DIR = "$tmp\$Phase\sendto"
    $env:COPYAPP_NO_BROWSER = '1'; $env:COPYAPP_NO_AUTOSTART_WRITE = '1'
    $env:COPYAPP_SKIP_PROMPTS = 'yes'; $env:COPYAPP_NO_OSD = '1'; $env:COPYAPP_WIFI_NAME = 'Test Wi-Fi'
    New-Item -ItemType Directory -Force $env:COPYAPP_DATA_DIR, $env:COPYAPP_RECEIVE_DIR | Out-Null
    '{"language":"en"}' | Set-Content -LiteralPath "$env:COPYAPP_DATA_DIR\settings.json"
}
function Connect-Control {
    $script:dd = $env:COPYAPP_DATA_DIR
    Wait-Until 'authenticated local control ready' {
        try {
            $script:lh = @{ 'X-CopyApp' = (Get-Content -LiteralPath "$dd\local.key" -Raw).Trim() }
            $script:L = "http://127.0.0.1:$((Get-Content -LiteralPath "$dd\local-port.txt" -Raw).Trim())"
            $script:state = Invoke-RestMethod "$L/local/state" -Headers $lh -TimeoutSec 2
            return $true
        } catch { return $false }
    }
    foreach ($owned in Owned-Processes) { Assert-LoopbackListeners $owned.Id }
}
function Pair-Phone {
    $null = Invoke-WebRequest "$L/pairing/open" -Method POST -Headers (@{ Origin=$L; 'X-CopyApp'=$lh.'X-CopyApp' }) -TimeoutSec 5
    Invoke-WebRequest "$L/qr" -Headers $lh -OutFile "$tmp\qr.png" -TimeoutSec 5 | Out-Null
    $pairUrl = & node "$PSScriptRoot\qr-decode.js" "$tmp\qr.png"
    if ($LASTEXITCODE -or $pairUrl -notmatch '^https://[^/]+/pair\?t=[a-f0-9]{64}(#fp=[a-f0-9]{64})?$') { throw 'QR decoding failed' }
    $script:B = ([uri]$pairUrl).GetLeftPart([UriPartial]::Authority)
    $null = Invoke-WebRequest $pairUrl -SkipCertificateCheck -SessionVariable phone -TimeoutSec 10
    $cookie = $phone.Cookies.GetAllCookies() | Where-Object Name -eq 'copyapp_session'
    Assert 'paired phone session issued' ([bool]$cookie)
    $script:phoneHeaders = @{ Cookie = "copyapp_session=$($cookie.Value)" }
}
Add-Type -AssemblyName System.Windows.Forms
Add-Type -ReferencedAssemblies System.Windows.Forms,System.Threading.Thread,System.Threading -TypeDefinition @'
using System;
using System.Threading;
using System.Runtime.InteropServices;
using System.Windows.Forms;
public static class DesktopHotkeyProbe {
 [StructLayout(LayoutKind.Sequential)] struct MSG { public IntPtr hwnd; public uint message; public UIntPtr wParam; public IntPtr lParam; public uint time; public int x,y; public uint priv; }
 [DllImport("user32.dll", SetLastError=true)] static extern bool RegisterHotKey(IntPtr h,int id,uint mods,uint key);
 [DllImport("user32.dll")] static extern bool UnregisterHotKey(IntPtr h,int id);
 [DllImport("user32.dll")] static extern bool PeekMessage(out MSG msg,IntPtr h,uint min,uint max,uint remove);
 [DllImport("user32.dll")] static extern bool TranslateMessage(ref MSG msg);
 [DllImport("user32.dll")] static extern IntPtr DispatchMessage(ref MSG msg);
 [DllImport("user32.dll")] public static extern void keybd_event(byte key,byte scan,uint flags,UIntPtr extra);
 static Thread thread; static volatile bool stop; static volatile string pending;
 static ManualResetEvent ready = new ManualResetEvent(false), written = new ManualResetEvent(false);
 static Exception error;
 public static void Start() {
  stop=false; error=null; ready.Reset();
  thread=new Thread(Run); thread.IsBackground=true; thread.SetApartmentState(ApartmentState.STA); thread.Start();
  if(!ready.WaitOne(10000)) throw new Exception("Hotkey reservation thread timed out");
  if(error!=null) throw new Exception("Cannot reserve Ctrl+Alt+C",error);
 }
 static void Run() {
  DataObject saved=new DataObject(); bool registered=false;
  try {
   var old=Clipboard.GetDataObject();
   if(old!=null) foreach(var format in old.GetFormats(false)) { try { var value=old.GetData(format,false); if(value!=null) saved.SetData(format,false,value); } catch {} }
   registered=RegisterHotKey(IntPtr.Zero,0x6CA1,0x0001|0x0002|0x4000,0x43);
   if(!registered) throw new Exception("RegisterHotKey failed: "+Marshal.GetLastWin32Error());
   ready.Set();
   while(!stop) {
    var text=pending; if(text!=null) { pending=null; Clipboard.SetText(text); written.Set(); }
    MSG msg; while(PeekMessage(out msg,IntPtr.Zero,0,0,1)) { TranslateMessage(ref msg); DispatchMessage(ref msg); }
    Thread.Sleep(10);
   }
  } catch(Exception e) { error=e; ready.Set(); written.Set(); }
  finally { if(registered) UnregisterHotKey(IntPtr.Zero,0x6CA1); try { Clipboard.SetDataObject(saved,true); } catch {} }
 }
 public static void SetText(string text) { written.Reset(); pending=text; if(!written.WaitOne(5000)||error!=null) throw new Exception("Test clipboard write failed",error); }
 public static void Stop() { stop=true; if(thread!=null&&!thread.Join(10000)) throw new Exception("Hotkey reservation thread did not exit"); }
 public static void PressFallback() {
  byte[] keys={0x11,0x12,0x10,0x43};
  try { foreach(byte k in keys) keybd_event(k,0,0,UIntPtr.Zero); Thread.Sleep(120); }
  finally { Array.Reverse(keys); foreach(byte k in keys) keybd_event(k,0,2,UIntPtr.Zero); }
 }
}
'@
$probeStarted = $false
try {
    New-Item -ItemType Directory -Force "$tmp\portable", "$tmp\download" | Out-Null
    $env:NODE_PATH = $NodeModules
    Configure 'fallback'; $env:COPYAPP_PORTABLE = '1'
    Copy-Item -LiteralPath $Exe -Destination "$tmp\portable\copyapp.exe"
    [DesktopHotkeyProbe]::Start(); $probeStarted = $true
    $portable = Start-Process "$tmp\portable\copyapp.exe" -WindowStyle Hidden -PassThru
    Connect-Control
    Wait-Until 'occupied primary hotkey selects Ctrl+Alt+Shift+C' {
        (Invoke-RestMethod "$L/local/state" -Headers $lh -TimeoutSec 2).hotkey -eq 'Ctrl+Alt+Shift+C'
    }
    Pair-Phone
    $marker = 'desktop-fallback-' + [guid]::NewGuid().ToString('N')
    [DesktopHotkeyProbe]::SetText($marker)
    [DesktopHotkeyProbe]::PressFallback()
    Wait-Until 'real fallback keypress publishes clipboard' {
        $events = & curl.exe -sk --max-time 2 -H "Cookie: $($phoneHeaders.Cookie)" "$B/events" 2>$null
        "$events" -match [regex]::Escape($marker)
    }
    $log = Get-Content -LiteralPath "$dd\copyapp.log" -Raw
    Assert 'fallback failure, registration and actual keypress logged' ($log -match 'hotkey candidate=1 registration failed' -and $log -match 'hotkey registered: Ctrl\+Alt\+Shift\+C' -and $log -match 'hotkey received: Ctrl\+Alt\+Shift\+C')
    $null = Invoke-WebRequest "$L/local/quit" -Headers $lh -Method POST -TimeoutSec 5
    Wait-Until 'portable test process exits' { $portable.Refresh(); $portable.HasExited }
    [DesktopHotkeyProbe]::Stop(); $probeStarted = $false

    Configure 'upgrade'; $env:COPYAPP_PORTABLE = $null
    Copy-Item -LiteralPath $BaselineExe -Destination "$tmp\download\copyapp.exe"
    $download = Start-Process "$tmp\download\copyapp.exe" -WindowStyle Hidden -PassThru
    Connect-Control
    Wait-Until 'installed executable and Desktop shortcut ready' { (Test-Path "$env:COPYAPP_INSTALL_DIR\copyapp.exe") -and (Test-Path "$env:COPYAPP_DESKTOP_DIR\CopyApp.lnk") }
    $installed = "$env:COPYAPP_INSTALL_DIR\copyapp.exe"
    $old = @(Owned-Processes | Where-Object Path -eq $installed)
    Assert 'one installed instance running' ($old.Count -eq 1)
    $oldProcess = $old[0]; $oldHash = (Get-FileHash -LiteralPath $installed).Hash
    Pair-Phone
    $saved = @{ receiveMode='folder'; saveDir=$env:COPYAPP_RECEIVE_DIR; maxBytes=524288000; language='en' } | ConvertTo-Json
    $null = Invoke-RestMethod "$L/local/settings" -Headers $lh -Method POST -ContentType application/json -Body $saved -TimeoutSec 5
    $oldKey = $lh.'X-CopyApp'
    $updated = "$tmp\download\updated-copyapp.exe"
    Copy-Item -LiteralPath $Exe -Destination $updated
    $stream = [IO.File]::Open($updated, [IO.FileMode]::Append, [IO.FileAccess]::Write)
    try { $bytes = [Text.Encoding]::ASCII.GetBytes('CopyApp isolated upgrade overlay ' + [guid]::NewGuid().ToString('N')); $stream.Write($bytes,0,$bytes.Length) } finally { $stream.Dispose() }
    $newHash = (Get-FileHash -LiteralPath $updated).Hash
    Assert 'upgrade fixture differs in binary content' ($oldHash -ne $newHash)
    $updateProcess = Start-Process $updated -WindowStyle Hidden -PassThru
    Wait-Until 'upgrade stops previous installed process' { $oldProcess.Refresh(); $oldProcess.HasExited }
    Wait-Until 'installed binary replaced with updated content' { try { (Get-FileHash -LiteralPath $installed).Hash -eq $newHash } catch { $false } }
    Wait-Until 'upgrade rotates the local control secret' { try { (Get-Content "$dd\local.key" -Raw).Trim() -ne $oldKey } catch { $false } }
    Connect-Control
    $new = @(Owned-Processes | Where-Object Path -eq $installed)
    Assert 'one replacement instance has a new PID' ($new.Count -eq 1 -and $new[0].Id -ne $oldProcess.Id)
    $newProcess = $new[0]
    $st = Invoke-RestMethod "$L/local/state" -Headers $lh -TimeoutSec 5
    Assert 'upgrade preserves PC settings' ($st.settings.receiveMode -eq 'folder' -and $st.settings.maxBytes -eq 524288000 -and $st.settings.saveDir -eq $env:COPYAPP_RECEIVE_DIR)
    $phoneStatus = Invoke-WebRequest "$B/status" -Headers $phoneHeaders -SkipCertificateCheck -TimeoutSec 5
    Assert 'upgrade preserves paired phone session and remembered HTTPS port' ($phoneStatus.StatusCode -eq 200)
    $link = (New-Object -ComObject WScript.Shell).CreateShortcut("$env:COPYAPP_DESKTOP_DIR\CopyApp.lnk")
    Assert 'Desktop shortcut targets updated install with --open' ($link.TargetPath -eq $installed -and $link.Arguments -eq '--open')
    $shortcutProcess = Start-Process $link.TargetPath -ArgumentList $link.Arguments -WindowStyle Hidden -PassThru
    Assert 'Desktop launch hands off successfully' ($shortcutProcess.WaitForExit($TimeoutSec * 1000) -and $shortcutProcess.ExitCode -eq 0)
    Assert 'Desktop launch keeps same installed process' (@(Owned-Processes | Where-Object Path -eq $installed).Count -eq 1 -and -not $newProcess.HasExited)
    # Uninstall must preserve unrelated files even inside custom install/data directories.
    'keep install sibling' | Set-Content "$env:COPYAPP_INSTALL_DIR\unrelated.txt"
    'keep data sibling' | Set-Content "$dd\unrelated.txt"
    'keep received file' | Set-Content "$env:COPYAPP_RECEIVE_DIR\mine.txt"
    $uninstall = Invoke-WebRequest "$L/local/uninstall" -Method POST -Headers $lh -TimeoutSec 5
    Assert 'uninstall accepted' ($uninstall.StatusCode -eq 202)
    Wait-Until 'uninstall removes app executable and credentials' { -not (Test-Path $installed) -and -not (Test-Path "$dd\local.key") }
    Assert 'uninstall preserves unrelated install/data/received files' ((Test-Path "$env:COPYAPP_INSTALL_DIR\unrelated.txt") -and (Test-Path "$dd\unrelated.txt") -and (Test-Path "$env:COPYAPP_RECEIVE_DIR\mine.txt"))
    Assert 'uninstall removes only owned shortcuts' (-not (Test-Path "$env:COPYAPP_DESKTOP_DIR\CopyApp.lnk") -and -not (Test-Path "$env:COPYAPP_STARTMENU_DIR\CopyApp.lnk") -and -not (Test-Path "$env:COPYAPP_SENDTO_DIR\Phone (CopyApp).lnk"))
    Write-Host 'All native desktop integration checks passed' -ForegroundColor Green
} catch {
    Write-Error -ErrorAction Continue $_
    exit 1
} finally {
    Stop-Owned
    if ($probeStarted) { [DesktopHotkeyProbe]::Stop() }
    foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name, $before[$name]) }
    $resolved = [IO.Path]::GetFullPath($tmp)
    if (-not $resolved.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $resolved -Leaf) -notmatch '^copyapp-desktop-[a-f0-9]{32}$') { throw 'Unsafe desktop cleanup root' }
    Remove-Item -LiteralPath $resolved -Recurse -Force -ErrorAction SilentlyContinue
}
