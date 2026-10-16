<#
Captures ONLY the real green "Sent to phone" box (window class CopyAppSentOSD) of the running exe to a PNG
(only the owned, topmost window rectangle). Run it just before pressing the hotkey; it waits up to -TimeoutSec.
#>
param([Parameter(Mandatory)][string]$Out, [Parameter(Mandatory)][int]$ProcessId, [ValidateSet('','Ctrl+Alt+C','Ctrl+Alt+Shift+C')][string]$Hotkey = '', [int]$TimeoutSec = 15)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
Add-Type @'
using System; using System.Runtime.InteropServices;
public static class O {
  [DllImport("user32.dll")] public static extern IntPtr FindWindow(string c, string t);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint f);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32.dll")] public static extern int GetWindowLong(IntPtr h, int index);
  [DllImport("user32.dll")] public static extern void keybd_event(byte key, byte scan, uint flags, UIntPtr extra);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
}
'@
$h = [IntPtr]::Zero
if ($Hotkey) {
    # Trigger the real global shortcut only after native capture APIs are loaded.
    $keys = @(0x11, 0x12); if ($Hotkey.Contains('Shift')) { $keys += 0x10 }; $keys += 0x43
    try { foreach ($k in $keys) { [O]::keybd_event($k, 0, 0, [UIntPtr]::Zero) }; Start-Sleep -Milliseconds 120 }
    finally { [array]::Reverse($keys); foreach ($k in $keys) { [O]::keybd_event($k, 0, 2, [UIntPtr]::Zero) } }
}
for ($i = 0; $i -lt $TimeoutSec * 100; $i++) {
    $h = [O]::FindWindow('CopyAppSentOSD', $null)
    [uint32]$ownerId = 0
    if ($h -ne [IntPtr]::Zero) { [void][O]::GetWindowThreadProcessId($h, [ref]$ownerId); if ($ownerId -eq $ProcessId) { break } }
    $h = [IntPtr]::Zero; Start-Sleep -Milliseconds 10
}
if ($h -eq [IntPtr]::Zero) { throw 'green box did not appear' }
Start-Sleep -Milliseconds 250
if (-not [O]::IsWindowVisible($h) -or ([O]::GetWindowLong($h, -20) -band 8) -eq 0) { throw 'Owned green confirmation is not visible and topmost' }
$r = New-Object O+RECT
if (-not [O]::GetWindowRect($h, [ref]$r) -or $r.R -le $r.L -or $r.B -le $r.T) { throw 'Invalid green box bounds' }
$bmp = New-Object System.Drawing.Bitmap ($r.R - $r.L), ($r.B - $r.T)
$g = [System.Drawing.Graphics]::FromImage($bmp)
try {
    $g.CopyFromScreen($r.L, $r.T, 0, 0, $bmp.Size)
    $bmp.Save($Out, [System.Drawing.Imaging.ImageFormat]::Png)
} finally { $g.Dispose(); $bmp.Dispose() }
"captured $($r.R-$r.L)x$($r.B-$r.T)"
