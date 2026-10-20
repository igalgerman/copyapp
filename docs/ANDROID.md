# Android companion (development; physical-device validation pending)

This small native app sends **text and URLs** to the existing Windows clipboard.
It does not monitor the clipboard in the background. The Quick Settings tile
opens a small visible Activity, waits for window focus, reads one text item, then
sends it over HTTPS. Successful sends close the Activity. Android 34+ uses the
PendingIntent form of `startActivityAndCollapse`; older supported versions use
the legacy API. Locked devices must be unlocked first. The tiny send Activity
handles orientation/screen-size changes without recreation and uses singleTask
launching to reuse an in-progress screen on repeated tile taps. A process-wide
in-flight guard prevents overlapping requests; restored saved Activity state
requires explicit retry rather than silently resending an uncertain transfer.
Completed requests never update a destroyed Activity.

## Build and installation

The `Android companion` GitHub Actions workflow builds/lints every pull request,
master push and `v*` tag using JDK 17, Gradle 8.11.1, Android SDK 35, AGP 8.9.2 and
Kotlin 2.1.20. It uploads `CopyApp-Android-development` containing an installable
**debug APK**. For local builds, install those tools and run from `android/`:

```sh
gradle --no-daemon :app:assembleDebug :app:lintDebug
```

The development APK compiled and passed Android lint in the
[PR #5 workflow](https://github.com/igalgerman/copyapp/actions/runs/39414876777).
It has not been installed or exercised on a physical phone.
The development APK is not a published friend-facing release. Its CI-generated
debug signing key is not stable across builds: uninstalling an older debug APK
may be required, which also removes pairing. Before publishing an APK as a GitHub
Release asset alongside Windows, the owner must provide a stable signing key,
wire release signing, and complete the physical-device checklist below. Never
commit a keystore or password. The current workflow deliberately publishes only
development artifacts; release delivery remains outstanding.

On the phone, download the artifact and extract/install the APK. Android asks
whether the browser/file manager may install unknown apps: allow this only for
this installation. Do not disable Play Protect. Device administrator policies
may prohibit sideloading.

## Pair once

1. Start the updated Windows CopyApp on the same Wi-Fi. Open a **fresh** pairing
   QR; opening a new QR does not revoke an existing paired browser.
2. Open native **CopyApp → Scan PC QR code** and grant camera permission. Compare
   the **entire SHA-256 fingerprint** shown on the phone with the full fingerprint
   in the Windows pairing window. Tap **Fingerprints match** only if identical.
3. Edit Android Quick Settings and add **CopyApp: Send to PC**. Copy demo text from
   another app, tap the tile, then press Ctrl+V in a Windows text editor. The main
   app also has **Send clipboard to PC**, using the same focused Activity.

The QR carries the PC's public CA certificate in its URL fragment. The app trusts
only that CA after explicit fingerprint confirmation, uses the platform X.509
trust manager and retains hostname verification. It never accepts arbitrary TLS
certificates. A QR from an older Windows version will ask you to update CopyApp.
The credential is returned only over the verified HTTPS connection and stored in
app-private storage with Android backup disabled. Tokens, text and credentials
are not logged.

Native pairing consumes a fresh one-use token into a **separate credential**.
The existing browser session stays valid, including across Windows restarts.
Pairing a replacement companion revokes the old companion; Windows **Unpair**
revokes both clients. Browser credentials cannot authorize the companion send
endpoint, and native credentials cannot authorize browser or local control routes.
The original session file format remains readable; upgraded files add optional
companion fields. Downgrading to an old CopyApp version drops companion support.

## Everyday use and recovery

Copy text → swipe down → **CopyApp: Send to PC** → **Sent to PC** → Windows Ctrl+V.
The request explicitly selects `mode=clipboard`, even if Windows normally saves
incoming items to a folder. Success requires a 2xx server response whose `where`
is `clipboard`. Failure keeps a visible Retry/Close dialog. A failed response can
be ambiguous if the PC accepted the text before Wi-Fi dropped; retrying text is
safe. The clipboard is read again on retry, so the user can re-copy expired text.

If the clipboard is empty, expired or an image, copy text again. For images and
files use the existing PWA **Share/image** flow; the native tile does not claim
image support. Open CopyApp again and scan a fresh QR after the PC's IP changes,
credentials expire or Windows revokes pairing. **Forget this PC on phone** clears
local data; use Windows Unpair to revoke the server credential as well.

The server logs companion pairing, accepted sends and existing receive outcome
stages without contents. Native focus/read/network failures currently appear in
the phone's visible error dialog; they do **not** upload directly to Sentry.
Native remote diagnostic integration remains an explicit limitation.

## Required physical-device validation (not yet performed)

- Samsung phone and supported Android 13/14/15/16: install, scan the enlarged QR,
  compare fingerprint, add tile, copy in WhatsApp/Chrome, tile → PC Ctrl+V without
  opening the PWA or a confirmation screen after pairing.
- Test tile from locked/unlocked screen, English/Hebrew text, empty/expired
  clipboard, denied clipboard/camera access, repeated taps and focus acquisition.
- Confirm a wrong fingerprint is rejected by the user, an unrelated/expired CA
  or wrong hostname fails TLS, and a wrong/expired/reused token cannot pair.
- Verify PWA before/after native pairing, restart, Windows Unpair, browser re-pair
  and companion replacement. Confirm companion credential cannot access `/events`.
- Check offline PC, mobile network versus Wi-Fi, changed PC IP, server/clipboard
  error and timeouts: no false success and an actionable retry/re-pair flow.
- Run APK build/lint, install the **signed release** APK on Samsung, and verify
  safe upgrade with the same signing key before publishing release/tutorials.
