# CopyApp 1.0.0 — 2026-10-15

Download and videos: https://copyapp.igalg.chatgpt.site/

## Changes

- Installs for the current Windows user, with Desktop, Start menu and Send to shortcuts. Opening the shortcut opens the control window, even after quitting.
- Asks whether to start with Windows; the choice can be changed later.
- PC control window: connection status, pairing, destination, receive folder, size limit, language, startup setting and uninstall.
- Phone: saved Auto / Clipboard / File destination and automatic text-copy / file-download preferences.
- Default file limit 2 GiB; large files stream to disk. Upload progress includes speed, ETA and cancellation. Browser-native Android downloads avoid buffering a multi-gigabyte Blob; browser Downloads shows their progress. Browsers with a save-file picker support in-page streamed download progress.
- PC clipboard shortcut logs registration and attempts, gives beep/tray feedback and uses Ctrl+Alt+Shift+C if Ctrl+Alt+C is unavailable. The tray menu remains a fallback.
- Fixed explicit clipboard destination for multipart text and URLs. Initial inbox replay does not repeat automatic receive actions.
- Safe uninstall preserves received and unrelated files. Local control actions retain loopback, secret and Origin checks. Queued files are checked against the current size limit before serving.

## Verification

- Project-wide `go vet ./...` and `go test -count=1 ./...` passed, including phone JavaScript behavior, translations and service worker checks.
- Installer lifecycle tests cover shortcut creation, app replacement and preserving unrelated files.
- Real executable end-to-end checks passed, including actual global hotkey PC-to-phone delivery, destination modes, 2 GiB default, Range, session/settings persistence and unpairing.
- Public recordings use `Home Wi-Fi`; mock platform screens are labelled as illustrations.
- Still requires a real Android check of clipboard permissions, downloads, the optional certificate/app installation and Android Share menu. Browser automation cannot establish those device-specific behaviors.

## SHA-256

`copyapp.exe`: `07b84601ff5d9e5edded6a1bb407a341b07d81dac564d4a83964b2f9b3b3f17d`

## Rulings I made

- Kept the existing architecture rather than adding a separate spike.
- Kept certificate installation optional: browser pairing works first; installing the app and Share-menu support is an optional upgrade.
- Mobile data is a troubleshooting tip, not a required setup step.
- Pairing stays single-use with a ten-minute window.
- Preserved DPAPI key protection and the per-launch local secret.
- Default file size is 2 GiB, with bounded clipboard memory and disk-streamed files.
- Did not add rate limiting or session expiry in this batch; these remain known limitations.
- Kept direct LAN-IP pairing; mDNS remains dropped. Re-scan after a network/IP change.
- Masked the Wi-Fi name in public recordings and replaced the private fixture in tests.

## Publication

The public website hosts the executable and both videos. Public GitHub source/repository creation was rejected by automatic approval review and needs the user's separate approval; no source was uploaded to GitHub.
