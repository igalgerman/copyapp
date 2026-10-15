# CopyApp release notes

The full list of changes is in [CHANGELOG.md](../CHANGELOG.md).

Each release on GitHub (https://github.com/igalgerman/copyapp/releases) carries `copyapp.exe` and `SHA256SUMS`. Check a download in PowerShell:

```
Get-FileHash .\copyapp.exe -Algorithm SHA256
```

and compare it with the line in `SHA256SUMS`.

Still to be checked on real Android devices for each release: clipboard permissions in a browser tab, large downloads, the optional certificate/app installation and the Android Share menu (browser automation cannot cover these).
