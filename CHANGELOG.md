# Changelog

All notable changes to CopyApp. Versions follow [Semantic Versioning](https://semver.org/).

## Unreleased

- Added an optional native Android text/URL clipboard tile alongside the existing browser/PWA. Separate persisted credentials keep both paired; Windows Unpair revokes both.
- Native onboarding verifies the PC certificate fingerprint and pins its CA while retaining hostname validation. Clipboard reads wait for a focused Activity; overlapping sends and silent resend after recreation are prevented.
- Windows drains both hotkey event streams, retries unavailable registration and closed listeners, preserves clipboard access errors and retries transient failures. Actual shortcut and last-attempt status are visible; queued/published status does not claim Android receipt.
- Added optional, content-free hosted Sentry diagnostics with Support ID correlation, bounded queues/rates and local log fallback. Releases remain usable without a DSN.
- Added reproducible dependency locks, Windows/Android CI downloads and owner-key signed Android release automation. Stable APK publication requires owner signing setup and physical-device validation.
- Website distinguishes the supported browser/PWA flow from the Android development preview in Hebrew, English and Russian. Existing videos are labeled as v1.0.2 walkthroughs with illustrated OS scenes; four new real-device Hebrew recordings remain outstanding.

Physical Windows/Samsung acceptance, live Sentry provisioning and final tutorial recordings have not been completed by automated CI.

## [1.0.2] - 2026-10-16

- Automated tests and recordings use a separate loopback-only build, including installed/upgraded copies, to avoid repeated Windows Firewall prompts from temporary executable paths. Harnesses refuse production EXEs before launch and verify local-only listeners. Normal releases retain LAN access.

## [1.0.1] - 2026-10-16

### Fixed
- PC control sessions use an explicit request header with an origin-scoped browser token in `sessionStorage`. Cookies shared between localhost ports no longer authorize control requests; reloading the control window retains its session.
- Unpairing clears the pending PC outbox. Replacing an existing phone also clears old pending items and closes the previous phone's event streams.
- The phone reports a failed disconnect when the PC is unreachable or rejects the request, instead of falsely reporting success.
- Revocation is saved before removing the old session, so Windows file locks cannot restore an unpaired phone after restart. Pairing persistence failures issue no cookie; unpair persistence failures display explicit retry guidance.
- Receive-folder checks resolve existing junctions, symbolic links, mapped drives and short paths before applying the local/system/Startup restrictions. Unsafe saved destinations fall back to the safe default.
- If Downloads is redirected to an unsafe destination, received files default to `%LOCALAPPDATA%\CopyApp\Received`; startup refuses to continue if neither default is safe.
- Uninstall and graceful upgrade wait for background shortcut setup to finish, preventing shortcuts from being recreated after removal.

### Release and validation
- Product downloads and videos refer to the same versioned `v1.0.1` GitHub release assets. CI preserves assets in an already-published release and keeps its own build as a workflow artifact.
- Added browser integration coverage for real control authorization/reload, QR pairing, translations/layout, destination settings, clipboard round trips, file upload/download, size rejection and disconnect failures.
- Device-specific Android certificate installation, PWA installation and Share-menu behavior still require testing on the target phone. A normal Chrome tab remains available without installing the certificate or app.

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
