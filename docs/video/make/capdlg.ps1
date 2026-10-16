<#
Captures ONLY the native "Start CopyApp with Windows?" message box of the real exe (the owned window rectangle,
not the screen) to a PNG, then answers it with Yes (WM_COMMAND IDYES). Used by make.ps1 for install.mp4.
  capdlg.ps1 -Out file.png -TimeoutSec 20
#>
param([Parameter(Mandatory)][string]$Out, [Parameter(Mandatory)][string]$ExeRoot, [int]$TimeoutSec = 25)
$ErrorActionPreference = 'Stop'
$exePrefix = [IO.Path]::GetFullPath($ExeRoot).TrimEnd('\') + '\'
Add-Type -AssemblyName System.Drawing
Add-Type @'
using System; using System.Runtime.InteropServices; using System.Text;
public static class W {
  [DllImport("user32.dll")] public static extern IntPtr FindWindow(string c, string t);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint f);
  [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr h, uint m, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int cmd);
  [DllImport("user32.dll")] public static extern bool AttachThreadInput(uint a, uint b, bool attach);
  [DllImport("kernel32.dll")] public static extern uint GetCurrentThreadId();
  [DllImport("user32.dll")] public static extern void keybd_event(byte key, byte scan, uint flags, UIntPtr extra);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder text, int count);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h, StringBuilder text, int count);
  [DllImport("user32.dll")] public static extern IntPtr GetAncestor(IntPtr h, uint flag);
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X,Y; public POINT(int x,int y){X=x;Y=y;} }
  [DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(POINT p);
  [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr h, IntPtr after, int x,int y,int cx,int cy,uint flags);
  public static bool OwnsRectangle(IntPtr h, RECT r) {
    for(int y=r.T;y<r.B;y+=4) for(int x=r.L;x<r.R;x+=4)
      if(GetAncestor(WindowFromPoint(new POINT(x,y)),2)!=h) return false;
    return GetAncestor(WindowFromPoint(new POINT(r.R-1,r.B-1)),2)==h;
  }
  public delegate bool EnumProc(IntPtr h, IntPtr data);
  [DllImport("user32.dll")] public static extern bool EnumChildWindows(IntPtr h, EnumProc callback, IntPtr data);
  public static string Describe(IntPtr h) {
    var text = new StringBuilder(4096); var cls = new StringBuilder(256);
    GetWindowText(h, text, text.Capacity); GetClassName(h, cls, cls.Capacity);
    return cls.ToString() + ": " + text.ToString();
  }
  [DllImport("dwmapi.dll")] public static extern int DwmGetWindowAttribute(IntPtr h, uint attr, out RECT r, uint size);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
}
'@
$h = [IntPtr]::Zero
for ($i = 0; $i -lt $TimeoutSec * 5; $i++) {
    $h = [W]::FindWindow('#32770', 'CopyApp')
    if ($h -ne [IntPtr]::Zero) { [uint32]$pid2 = 0; [void][W]::GetWindowThreadProcessId($h, [ref]$pid2); $p = Get-Process -Id $pid2 -ErrorAction SilentlyContinue; if ($p -and $p.Path -and $p.Path.StartsWith($exePrefix, [StringComparison]::OrdinalIgnoreCase)) { break } }
    $h = [IntPtr]::Zero; Start-Sleep -Milliseconds 200
}
if ($h -eq [IntPtr]::Zero) { throw 'startup question did not appear' }
Start-Sleep -Milliseconds 600
$childDescriptions = [Collections.Generic.List[string]]::new()
$enumerate = [W+EnumProc]{ param($child, $data) $childDescriptions.Add([W]::Describe($child)); return $true }
[void][W]::EnumChildWindows($h, $enumerate, [IntPtr]::Zero)
Write-Host ('Owned native dialog controls: ' + ($childDescriptions -join ' | '))
[void][W]::ShowWindow($h, 9)
$captureThread = [W]::GetCurrentThreadId()
[uint32]$foregroundOwner = 0
$foregroundThread = [W]::GetWindowThreadProcessId([W]::GetForegroundWindow(), [ref]$foregroundOwner)
$attached = $foregroundThread -ne 0 -and $foregroundThread -ne $captureThread -and [W]::AttachThreadInput($captureThread, $foregroundThread, $true)
try {
    # An Alt transition grants the current interactive input thread permission
    # to activate the owned dialog under Windows foreground-lock rules.
    [W]::keybd_event(0x12, 0, 0, [UIntPtr]::Zero)
    try { [void][W]::SetForegroundWindow($h) }
    finally { [W]::keybd_event(0x12, 0, 2, [UIntPtr]::Zero) }
}
finally { if ($attached) { [void][W]::AttachThreadInput($captureThread, $foregroundThread, $false) } }
Start-Sleep -Milliseconds 300
if (-not [W]::IsWindowVisible($h)) {
    $fg = [W]::GetForegroundWindow(); [uint32]$fgPid = 0; [void][W]::GetWindowThreadProcessId($fg, [ref]$fgPid)
    $fgClass = [Text.StringBuilder]::new(256); [void][W]::GetClassName($fg, $fgClass, 256)
    Write-Host "Foreground metadata: pid=$fgPid process=$((Get-Process -Id $fgPid -ErrorAction SilentlyContinue).ProcessName) class=$fgClass rootowner=$([W]::GetAncestor($fg,3)); dialog rootowner=$([W]::GetAncestor($h,3))"
    throw "Owned startup dialog is not the visible foreground window (visible=$([W]::IsWindowVisible($h)); foreground=$fg; dialog=$h)"
}
$r = New-Object W+RECT
if ([W]::DwmGetWindowAttribute($h, 9, [ref]$r, 16) -ne 0 -or $r.R -le $r.L -or $r.B -le $r.T) { throw 'Invalid visible startup dialog bounds' }
# Keep the rounded outer border/shadow out of the crop, and prove every 4px
# sample belongs to the owned native dialog, before and after taking pixels.
$r.L += 8; $r.R -= 8; $r.T += 8; $r.B -= 8
if (-not [W]::SetWindowPos($h, [IntPtr](-1), 0,0,0,0, 0x53)) { throw 'Cannot make owned startup dialog topmost' }
Start-Sleep -Milliseconds 200
if (-not [W]::OwnsRectangle($h,$r)) { throw 'Owned startup dialog crop is occluded' }
$bmp = New-Object System.Drawing.Bitmap ($r.R - $r.L), ($r.B - $r.T)
$g = [System.Drawing.Graphics]::FromImage($bmp)
try {
    # PrintWindow omits message-box static text on current Windows builds.
    # Capture only the process-verified, unoccluded native window interior.
    $g.CopyFromScreen($r.L, $r.T, 0, 0, $bmp.Size)
    if (-not [W]::OwnsRectangle($h,$r)) { throw 'Owned startup dialog was occluded during capture' }
    $bmp.Save($Out, [System.Drawing.Imaging.ImageFormat]::Png)
} finally { $g.Dispose(); $bmp.Dispose() }
[void][W]::PostMessage($h, 0x111, [IntPtr]6, [IntPtr]::Zero)   # WM_COMMAND, IDYES
"captured $($r.R-$r.L)x$($r.B-$r.T)"
