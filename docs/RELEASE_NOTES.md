# CopyApp release notes

The full list of changes is in [CHANGELOG.md](../CHANGELOG.md).

Each release on GitHub (https://github.com/igalgerman/copyapp/releases) carries `copyapp.exe`, `install.mp4`, `use.mp4` and `SHA256SUMS`. Check a download in PowerShell:

```
Get-FileHash .\copyapp.exe -Algorithm SHA256
```

and compare it with the line in `SHA256SUMS`.

After building and testing the release executable, regenerate tutorials with `pwsh -File docs/video/make/make.ps1 -NoBuild -IllustrateNative`. This keeps the tested EXE unchanged and labels the Windows startup/green-confirmation illustrations. Real app, pairing and transfer footage is recorded. Public examples mask Wi-Fi/address/path labels and the displayed QR. The default guarded native-window capture mode and `-StartupPng` are documented in the script header.

Still to be checked on real Android devices for each release: clipboard permissions in a browser tab, large downloads, the optional certificate/app installation and the Android Share menu (browser automation cannot cover these).
