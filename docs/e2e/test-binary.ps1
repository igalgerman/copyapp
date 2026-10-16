# Inspect build metadata without starting an app that could open a LAN listener.
function Assert-LoopbackTestBinary([string]$Path) {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { $env:PATH = "C:\Program Files\Go\bin;$env:PATH" }
    $metadata = & go version -m $Path 2>&1
    if ($LASTEXITCODE -or ($metadata -join "`n") -notmatch '(?m)^\s*build\s+-tags=copyapp_test_loopback\s*$') {
        throw "Tests require a loopback-only binary. Run build.ps1 -LoopbackTest; release EXEs are refused to avoid repeated firewall prompts. Binary: $Path"
    }
}

function Assert-LoopbackListeners([int]$ProcessId) {
    $listeners = @(Get-NetTCPConnection -OwningProcess $ProcessId -State Listen -ErrorAction Stop)
    if ($listeners.Count -lt 2 -or @($listeners | Where-Object LocalAddress -ne '127.0.0.1').Count) {
        throw "Test process $ProcessId must listen only on 127.0.0.1"
    }
    Write-Host 'PASS  test listeners are restricted to this PC'
}
