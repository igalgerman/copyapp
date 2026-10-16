<# Real 2 GiB upload, disk-backed download/resume and interrupted-upload checks.
Invoked by run.ps1 -LargeFiles inside its isolated app/receive directories. #>
param([string]$BaseURL,[string]$LocalURL,[string]$LocalKey,[string]$Cookie,[string]$WorkDir,[string]$ReceiveDir)
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath($WorkDir).TrimEnd('\')
$tempRoot = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
if (-not $root.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or
    -not [IO.Path]::GetFullPath($ReceiveDir).StartsWith($root + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe large-file test directories' }
if ((Get-PSDrive -Name ([IO.Path]::GetPathRoot($root).Substring(0,1))).Free -lt 5GB) { throw 'Large-file check needs 5 GB free disk space' }
$source = Join-Path $root 'large-review.bin'
$saved = Join-Path $ReceiveDir 'large-review.bin'
$cancelled = Join-Path $ReceiveDir 'cancelled-review.bin'
$client = $null; $stream = $null; $form = $null; $cts = $null
try {
    $opt = @{ SkipHttpErrorCheck=$true; TimeoutSec=30 }
    $r = Invoke-WebRequest "$LocalURL/local/settings" -Method POST -Headers @{ 'X-CopyApp'=$LocalKey } -ContentType application/json -Body '{"maxBytes":2147483648}' @opt
    if ($r.StatusCode -ne 200) { throw 'Could not set 2 GiB test limit' }
    $stream = [IO.File]::Open($source,[IO.FileMode]::Create,[IO.FileAccess]::ReadWrite,[IO.FileShare]::Read)
    $stream.SetLength(2GB)
    $stream.WriteByte(37); $stream.Position=2GB-65536; $stream.WriteByte(219)
    $stream.Dispose(); $stream=$null
    $handler = [Net.Http.HttpClientHandler]::new()
    $handler.ServerCertificateCustomValidationCallback = [Net.Http.HttpClientHandler]::DangerousAcceptAnyServerCertificateValidator
    $client = [Net.Http.HttpClient]::new($handler); $client.Timeout=[TimeSpan]::FromMinutes(20)
    $client.DefaultRequestHeaders.Add('Cookie',$Cookie)
    $form = [Net.Http.MultipartFormDataContent]::new()
    $form.Add([Net.Http.StringContent]::new('file'),'mode')
    $stream = [IO.File]::OpenRead($source)
    $part = [Net.Http.StreamContent]::new($stream,65536)
    $part.Headers.ContentType = [Net.Http.Headers.MediaTypeHeaderValue]::new('application/octet-stream')
    $form.Add($part,'file','large-review.bin')
    $response = $client.PostAsync("$BaseURL/send",$form).GetAwaiter().GetResult()
    $body = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    if ([int]$response.StatusCode -ne 200 -or -not (Test-Path -LiteralPath $saved) -or (Get-Item -LiteralPath $saved).Length -ne 2GB) { throw "2 GiB upload failed: $([int]$response.StatusCode) $body" }
    $response.Dispose(); $form.Dispose(); $form=$null; $stream=$null
    if ((Get-FileHash -LiteralPath $source).Hash -ne (Get-FileHash -LiteralPath $saved).Hash) { throw 'Large upload content differs' }
    Write-Host 'PASS  real 2 GiB upload and disk content hash'
    $r = Invoke-WebRequest "$LocalURL/local/send-file?path=$([uri]::EscapeDataString($source))" -Method POST -Headers @{ 'X-CopyApp'=$LocalKey } @opt
    if ($r.StatusCode -ne 204) { throw 'Could not queue the 2 GiB PC file' }
    $events = (& curl.exe -sk --max-time 2 -H "Cookie: $Cookie" "$BaseURL/events") -join "`n"
    $items = @($events -split "`n" | Where-Object { $_.StartsWith('data: ') } | ForEach-Object { $_.Substring(6) | ConvertFrom-Json } | Where-Object name -eq 'large-review.bin')
    if (-not $items.Count) { throw 'Large PC file missing from inbox' }
    $item = $items[-1]
    if ($item.size -ne 2GB) { throw 'Large PC file metadata has incorrect size' }
    foreach ($range in @('bytes=0-65535','bytes=2147418112-2147483647')) {
        $request = [Net.Http.HttpRequestMessage]::new([Net.Http.HttpMethod]::Get,"$BaseURL/item/$($item.id)")
        $request.Headers.TryAddWithoutValidation('Range',$range) | Out-Null
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        $bytes = $response.Content.ReadAsByteArrayAsync().GetAwaiter().GetResult()
        $start = if ($range -eq 'bytes=0-65535') { 0L } else { 2147418112L }
        $marker = if ($start -eq 0) { 37 } else { 219 }
        $cr = $response.Content.Headers.ContentRange
        if ([int]$response.StatusCode -ne 206 -or $bytes.Length -ne 65536 -or $cr.Length -ne 2GB -or $cr.From -ne $start -or $cr.To -ne ($start+65535) -or $bytes[0] -ne $marker -or ($bytes[1..65535] | Where-Object { $_ -ne 0 } | Select-Object -First 1)) { throw "Large download resume failed: $range" }
        $response.Dispose(); $request.Dispose()
    }
    Write-Host 'PASS  2 GiB disk-backed file: first and resumed final 64 KiB ranges'
    $status = & curl.exe -sk --max-time 10 -o (Join-Path $root 'oversize.json') -w '%{http_code}' -H "Cookie: $Cookie" -H 'Content-Type: multipart/form-data; boundary=limit' -H 'Content-Length: 2148532225' --data-binary '' "$BaseURL/send"
    if ($status -ne '413') { throw "Oversized request not rejected before upload: $status" }
    Write-Host 'PASS  over-limit request rejected before sending its body'
    Add-Type -TypeDefinition @'
using System; using System.IO; using System.Threading;
public sealed class SlowReviewStream : Stream {
 long remaining = 64L*1024*1024;
 public override bool CanRead=>true; public override bool CanSeek=>false; public override bool CanWrite=>false;
 public override long Length=>64L*1024*1024; public override long Position {get=>Length-remaining;set=>throw new NotSupportedException();}
 public override int Read(byte[] b,int o,int n){Thread.Sleep(5);n=(int)Math.Min(Math.Min(n,65536),remaining);Array.Clear(b,o,n);remaining-=n;return n;}
 public override void Flush(){} public override long Seek(long o,SeekOrigin s)=>throw new NotSupportedException();
 public override void SetLength(long v)=>throw new NotSupportedException();public override void Write(byte[] b,int o,int n)=>throw new NotSupportedException();
}
'@
    $form = [Net.Http.MultipartFormDataContent]::new()
    $form.Add([Net.Http.StringContent]::new('file'),'mode')
    $form.Add([Net.Http.StreamContent]::new([SlowReviewStream]::new(),65536),'file','cancelled-review.bin')
    $cts=[Threading.CancellationTokenSource]::new()
    $task=$client.PostAsync("$BaseURL/send",$form,$cts.Token)
    for($i=0;$i -lt 100 -and -not (Test-Path -LiteralPath $cancelled);$i++){Start-Sleep -Milliseconds 20}
    if (-not (Test-Path -LiteralPath $cancelled)) { throw 'Cancellation check never began writing the upload' }
    $cts.Cancel(); try {$task.GetAwaiter().GetResult() | Out-Null} catch {}
    $form.Dispose();$form=$null
    for($i=0;$i -lt 100 -and (Test-Path -LiteralPath $cancelled);$i++){Start-Sleep -Milliseconds 30}
    if (Test-Path -LiteralPath $cancelled) { throw 'Interrupted upload left a partial file' }
    Write-Host 'PASS  interrupted real upload removes its partial file'
}
finally {
    if($form){$form.Dispose()};if($stream){$stream.Dispose()};if($cts){$cts.Dispose()};if($client){$client.Dispose()}
    foreach($file in @($source,$saved,$cancelled)){Remove-Item -LiteralPath $file -Force -ErrorAction SilentlyContinue}
}
