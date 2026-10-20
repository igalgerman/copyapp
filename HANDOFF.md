# CopyApp handoff (2026-10-20, v1.0.3 closed)

Release v1.0.3 (tag `v1.0.3`) is published: Windows `copyapp.exe` (Sentry DSN embedded), owner-signed `CopyApp-Android.apk`, SHA256SUMS files and demo videos. Site: https://igalgerman.github.io/copyapp/ (GitHub Pages; the old Sites URL is retired and must be deleted by the owner). Do not move or re-run the v1.0.3 tag; ship fixes as a new version.

## Verified (automated, on a Windows 11 PC)
- go vet and all Go tests, site/diagnostics/JS tests, full real-EXE e2e (incl. 2 GiB files, browser, 20 separate Ctrl+Alt+C presses each sent once), native desktop integration (install/upgrade/uninstall, fallback hotkey), e2e with an unreachable Sentry host.
- Sentry project `copyapp` (org igorgerman) receives live events from the real EXE.
- Release assets hash-match their checksum files; APK is v2-signed with the owner key (SHA-256 F0:90:BE:F8:...:B6:97); all site links return 200; the APK download QR decodes to the APK URL.
- Fixed this session: pairing QR carried the whole CA and could not be scanned; it now carries only `#fp=<sha256>` and the Android app fetches/pins the public CA.

## NOT verified
- Any physical Android phone: APK install, pairing QR scan, Quick Settings tile, PWA coexistence. First thing to test.
- Hebrew/AltGr layouts, RDP, reboot for the hotkey; the original intermittent report on the owner's PC.
- Android-side Sentry events. Release EXE shows `vcs.modified=true` (CI-generated files).
- All videos on the site are illustrated demos (labelled), not real-device footage. Real Hebrew recordings: `docs/video/REAL-CAPTURE.he.md`.

## Owner housekeeping
- Back up `C:\Users\97254\copyapp-keys\` (release keystore + password). Losing it blocks APK upgrades. GitHub secrets COPYAPP_ANDROID_* and variables COPYAPP_ANDROID_CERT_SHA256, COPYAPP_SENTRY_DSN are set.
- Test builds use `build.ps1 -LoopbackTest`; never run temporary production copies repeatedly (Firewall prompts). E2E overwrites the Windows clipboard.
