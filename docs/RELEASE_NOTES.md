# CopyApp release notes

The full list of changes is in [CHANGELOG.md](../CHANGELOG.md).

Each release on GitHub (https://github.com/igalgerman/copyapp/releases) carries `copyapp.exe`, `install.mp4`, `use.mp4` and `SHA256SUMS`. Check a download in PowerShell:

```
Get-FileHash .\copyapp.exe -Algorithm SHA256
```

and compare it with the line in `SHA256SUMS`.

After building the release executable, build the same version with `build.ps1 -Version 1.0.2 -LoopbackTest` for local integration checks and recording. Regenerate tutorials with `pwsh -File docs/video/make/make.ps1 -Version 1.0.2 -NoBuild -IllustrateNative`. The separate test build restricts networking to this PC and does not overwrite the release EXE. Real app, pairing and transfer footage is recorded; Windows startup/green-confirmation illustrations are labeled. Public examples mask Wi-Fi/address/path labels and the displayed QR. The default guarded native-window capture mode and `-StartupPng` are documented in the script header.

Still to be checked on real Android devices for each release: clipboard permissions in a browser tab, large downloads, the optional certificate/app installation and the Android Share menu (browser automation cannot cover these).
