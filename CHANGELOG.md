# Changelog

All notable changes to CopyApp. Versions follow [Semantic Versioning](https://semver.org/).

## [1.0.0] - 2026-10-16

First public release.

### PC (Windows)
- Double-click `copyapp.exe`: it installs itself for the current user in `%LOCALAPPDATA%\Programs\CopyApp` (no administrator rights), creates Desktop, Start menu and Explorer "Send to → Phone (CopyApp)" shortcuts, asks once whether to start with Windows (Yes/No) and opens the CopyApp window with the pairing QR code and a "Before you scan" checklist naming the PC's Wi-Fi.
- The Desktop shortcut opens the window of the running copy. Started with Windows, CopyApp stays quietly in the tray.
- CopyApp window: phone status (with the address and time the phone paired), Wi-Fi, address, active shortcut, a "Send PC clipboard to phone now" button, transfers with progress, and settings: language, what to do with items from the phone (clipboard or folder), save folder, largest item (200 MB – 2 GB, default 2 GB), Start with Windows, Unpair phone, Uninstall.
- Ctrl+Alt+C sends the PC clipboard to the phone (Ctrl+Alt+Shift+C if the first is taken). Feedback: a sound, a green "✓ Sent to phone" box near the clock, a tray-icon tick and a notification; every attempt is logged.
- Uninstall removes only what CopyApp created; received files are always kept.
- Executable with icon, version information and a DPI-aware manifest; version shown in the window and the log.

### Phone (Android browser)
- Works in a normal Chrome tab after scanning the QR code; installing as an app (Share menu) is optional.
- Destination per send: Auto / Clipboard / File. Receive settings: auto-copy text, auto-download files.
- Files up to 2 GB stream to and from disk, with progress, speed, time left and cancel. Downloads support resuming (HTTP Range).
- Hebrew (default, right-to-left), Russian and English.

### Security
- Pairing token: 256-bit, single use, 10 minutes. Phone session: `HttpOnly`, `Secure`, `SameSite=Strict`, expires after 180 days without use.
- PC-only pages need a per-launch secret; the browser gets a separate in-memory session token.
- Own name-constrained certificate authority; TLS keys protected with DPAPI.
- Received files get the Mark of the Web; shortcut-like file types are renamed to `.blocked`; the save folder cannot be a network, system or Startup folder.
- A share that reaches the phone app is sent only after the user confirms it.
- Each pairing is announced on the PC with the phone's IP address.
- Logs never contain clipboard content, tokens or keys.

See [SECURITY.md](SECURITY.md) for the threat model and known limits.
