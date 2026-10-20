<#
Builds the release executable: dist\copyapp.exe (Windows, amd64).

  pwsh -File build.ps1 [-Version 1.0.0] [-Test]

It embeds the icon, version information and the application manifest
(DPI aware, runs as the normal user) from cmd\copyapp\winres\winres.json via
go-winres, builds without a console window, stamps the version shown in the
CopyApp window and the log, and prints the SHA-256 of the result.
-Test runs `go vet` and `go test` first.
-LoopbackTest builds output\test-bin\copyapp.exe with network access restricted
to this PC, for automated tests/recording without fresh LAN firewall prompts.
#>
param(
    [ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version = '1.0.2',
    [switch]$Test,
    [switch]$LoopbackTest,
    [ValidatePattern('^$|^https://[a-fA-F0-9]{16,64}@o[0-9]+\.ingest(?:\.(?:us|de))?\.sentry\.io/[0-9]+$')][string]$SentryDSN = $env:COPYAPP_SENTRY_DSN
)
$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { $env:PATH = "C:\Program Files\Go\bin;$env:PATH" }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go is not installed (https://go.dev/dl/)' }
$env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'

Push-Location $root
try {
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) { throw 'Node.js/npm is required to bundle browser diagnostics (https://nodejs.org/)' }
    npm install --ignore-scripts
    if ($LASTEXITCODE) { throw 'npm install failed' }
    npm run build:diagnostics
    if ($LASTEXITCODE) { throw 'Browser diagnostics bundle failed' }
    go mod tidy
    if ($LASTEXITCODE) { throw 'Go dependency resolution failed' }
    if ($Test) {
        go vet ./...; if ($LASTEXITCODE) { throw 'go vet failed' }
        go test -count=1 ./...; if ($LASTEXITCODE) { throw 'go test failed' }
    }
    # Icon, version info and manifest -> cmd\copyapp\rsrc_windows_amd64.syso
    go run github.com/tc-hib/go-winres@v0.3.3 make --in cmd/copyapp/winres/winres.json --out cmd/copyapp/rsrc `
        --arch amd64 --product-version "$Version.0" --file-version "$Version.0"
    if ($LASTEXITCODE) { throw 'go-winres failed' }
    $destination = if ($LoopbackTest) { 'output\test-bin' } else { 'dist' }
    $buildTags = if ($LoopbackTest) { @('-tags', 'copyapp_test_loopback') } else { @() }
    New-Item -ItemType Directory -Force $destination | Out-Null
    $linkFlags = "-H=windowsgui -s -w -X main.version=$Version"
    if ($SentryDSN) { $linkFlags += " -X github.com/igalgerman/copyapp/internal/diagnostics.BuildDSN=$SentryDSN" }
    go build @buildTags -trimpath -ldflags $linkFlags -o "$destination\copyapp.exe" ./cmd/copyapp
    if ($LASTEXITCODE) { throw 'go build failed' }
    $exe = Get-Item "$destination\copyapp.exe"
    $hash = (Get-FileHash $exe.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  copyapp.exe" | Set-Content -Encoding ascii "$destination\SHA256SUMS"
    Write-Host "Built $($exe.FullName) (CopyApp $Version, $($exe.Length) bytes)"
    Write-Host "SHA-256: $hash"
}
finally {
    Pop-Location
}
