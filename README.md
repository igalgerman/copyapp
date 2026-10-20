# CopyApp

**[עברית](README.he.md) · [Русский](README.ru.md) · English · [Website](https://copyapp.igalg.chatgpt.site/)**

CopyApp lets you copy and paste between your Windows PC and your Android phone.

Development work for issues #1–#4 is documented in [diagnostics](docs/DIAGNOSTICS.md), [Android companion](docs/ANDROID.md), [Windows hotkey verification](docs/HOTKEY-QA.md), and the [Hebrew real-device tutorial recording kit](docs/video/REAL-CAPTURE.he.md). Windows/Android builds pass CI; owner setup and physical-device validation remain required. The downloads below remain the previously published releases. Four new tutorial recordings are not yet available. The [updated website mirror](https://igalgerman.github.io/copyapp/) explains the browser/PWA and optional APK paths.

- Copy and paste **text**, **images**, and **files** between your PC and your phone.
- Transfers work over **your own Wi-Fi**; clipboard contents and files stay on your local network. Builds configured for Sentry send content-free diagnostic events to that service.
- It is **free** and open source ([MIT license](LICENSE)).

---

## Download

**Download `copyapp.exe` from the CopyApp website:**

https://copyapp.igalg.chatgpt.site/

or download the reviewed [CopyApp 1.0.2 executable](https://github.com/igalgerman/copyapp/releases/download/v1.0.2/copyapp.exe). The [1.0.2 release](https://github.com/igalgerman/copyapp/releases/tag/v1.0.2) includes `SHA256SUMS` to check the download.

---

## Install (3 steps)

> [!IMPORTANT]
> **Before you start**
> - Connect your phone to the **same Wi-Fi as your PC**. The CopyApp window shows the PC's Wi-Fi name.
> - A **guest Wi-Fi** or **"AP/client isolation"** on the router blocks the connection. Use the main Wi-Fi.
> - Pair only on **your own Wi-Fi** at home or at work, never on public Wi-Fi in a café or hotel.

1. **Download and run `copyapp.exe`** on your PC. CopyApp installs itself for your Windows user (no administrator rights needed) and creates Desktop and Start menu shortcuts. Answer **Yes** or **No** to the question about starting with Windows. The CopyApp window opens with a QR code; the CopyApp icon stays near the clock (it may be hidden under the **^** arrow).
2. **Scan the QR code** with your phone's camera and open the link.
3. **If the browser says "Your connection is not private", tap Advanced → Proceed only on your own trusted network.** The certificate is created by your PC. Bypassing the warning does not verify the PC's identity; see [Security](SECURITY.md).

That's it. CopyApp is now open in your phone's browser, ready to send and receive. Tip: bookmark the page or choose **Add to Home screen** in Chrome's ⋮ menu.

> The first time only, Windows may show **"Windows protected your PC"** (click **More info → Run anyway**: CopyApp is not digitally signed) and a **firewall prompt** (click **Allow**). Prompts may appear again after an update.

### Optional: install as an app (about 1 minute)

Want CopyApp in Android's **Share** menu and no "Not secure" warning? On the phone, open **⋯ → Install as an app** and follow the steps there:

1. On the PC, open the CopyApp window and expand **Certificate code**.
2. Check that the code on the phone is the same.
3. Download the certificate and install it once in Android settings (menu names differ a little between phone makers).
4. In Chrome's ⋮ menu choose **Install app**.

The certificate is restricted to private/local addresses and the `localhost`/`copyapp.local` names. You can remove it any time in Android settings. Android menus and installation support vary by device; the browser-tab flow remains available.

---

## How to use it

**Phone to PC**
- Choose where it goes on the PC:
  - **Auto**: as set in the CopyApp window on the PC.
  - **Clipboard**: text and PNG pictures go to the PC clipboard, ready for Ctrl+V.
  - **File**: saved in the PC's receive folder.
- Then tap **Send clipboard**, **Photo** or **File**. (If the browser won't share the clipboard, a text box opens: paste there and tap Send.)
- After the optional app install: use the Android **Share** menu on a text, image, or file and choose **CopyApp**. The phone asks **Send to your PC?** before anything is sent.

**PC to phone**
- Press the active shortcut shown in CopyApp (normally **Ctrl+Alt+C**) to send the PC clipboard. Current development builds distinguish **Queued on PC** from **Published for connected phone**; neither confirms the phone copied it. Check the item on the phone.
- Or click **Send clipboard to phone** in the CopyApp icon menu or in the CopyApp window.
- Or right-click a file and choose **Send to → Phone (CopyApp)**. On Windows 11, choose **Show more options** first.
- If another program already uses Ctrl+Alt+C, CopyApp uses **Ctrl+Alt+Shift+C** instead and tells you so.

### Settings

Open the **CopyApp** shortcut on the Desktop (or the icon near the clock) to open the CopyApp window. There you can choose what happens with items from the phone (clipboard or folder), the save folder, the largest file size, the language and Start with Windows. The phone's own settings (⋯) can copy text and download files automatically.

To remove CopyApp, choose **Uninstall CopyApp…** in the CopyApp window or in the icon menu. Files you received are never deleted. CopyApp is not listed under Windows "Installed apps".

## Good to know (limits)

- Android only lets a web page read the clipboard when you tap. So on the phone, you tap to copy an item.
- It only works when your PC and phone are on the **same Wi-Fi**.
- It works with **one phone** paired to one PC.
- Items sent from the PC can be downloaded for **10 minutes**.
- The largest item is **2 GB** by default. You can change it in the CopyApp window; large files are written straight to disk.
- Android uses Chrome's download manager for file progress and cancellation. Automatic clipboard copying/downloads can be blocked by browser permissions; use the item's buttons if needed. Device-specific certificate/app installation and the Android Share menu still need verification on your phone.
- The Windows executable is unsigned. Opening a different downloaded CopyApp binary can replace the installed one without a version/downgrade check. If the PC's LAN address changes, scan a fresh QR code.
- The app is available in **Hebrew** (default), **Russian** and **English**. Switch with **עב / РУ / EN** at the top of the phone screen and of the CopyApp window.

---

## Security

- All data sent between your PC and phone is **encrypted**.
- The pairing QR code can be used **only once** and works for **10 minutes**.
- When a phone pairs, the PC shows a notification with the phone's address, and the CopyApp window keeps showing it. If it was not you, choose **Unpair phone**.
- Transfers use **your local network**. CopyApp does not open your PC to the internet. Optional Sentry diagnostics use outbound HTTPS; see [diagnostics setup and data](docs/DIAGNOSTICS.md).
- Received files are saved to **Downloads\CopyApp** by default. If redirected Downloads is unsafe (for example, a network share), CopyApp uses `%LOCALAPPDATA%\CopyApp\Received`. The window shows the actual folder. Files are marked as "downloaded from the internet" where supported; executable files still need your judgment before opening.
- Unpairing clears pending PC items; replacing the existing paired phone also clears its pending outbox. A failed phone-side disconnect is reported as a failure, so retry it when the PC is reachable.
- You can **unpair at any time**: in the CopyApp window or the icon menu choose **Unpair phone**; on the phone choose **⋯ → Unpair this phone**.
- A phone that is not used for 180 days must pair again.

More details: [SECURITY.md](SECURITY.md).

---

## Troubleshooting

- **"Refused to connect", "took too long to respond" or the page never opens?** Check *Before you start*: same Wi-Fi as the PC, not a guest network. Then make sure the Windows firewall allows CopyApp (Windows Security → Firewall & network protection → Allow an app through firewall → tick CopyApp). If your Wi-Fi is marked **Public** in Windows, either switch it to **Private** (Settings → Network & internet → Wi-Fi → your network) or tick **Public** for CopyApp. The CopyApp window has the same tips under **Can't open the page?**
- **Pressed Ctrl+Alt+C and nothing happened?**
  - Check the active shortcut and last-send result in the CopyApp window. If the shortcut is unavailable, another program may have taken it; use **Send PC clipboard to phone now**. A clipboard-read failure has separate feedback.
  - The box appeared but the phone shows nothing: keep CopyApp open on the phone (the dot at the top says **Connected**).
  - The log file has one line per attempt (never the clipboard content): `%APPDATA%\CopyApp\copyapp.log`. Its path is also shown at the bottom of the CopyApp window.
- **Stuck on "not private" with no Advanced button?** Open the link in **Chrome**.
- **Your PC's IP address changed?** Choose **Pair phone** in the icon menu and scan the QR code again. Tip: reserve the PC's address in your router.
- **Still failing?** Try turning **mobile data off** briefly (some phones send traffic over mobile data when the Wi-Fi shows no internet).

---

## Videos

- [Install video — 1.0.2](https://github.com/igalgerman/copyapp/releases/download/v1.0.2/install.mp4)
- [Use video — 1.0.2](https://github.com/igalgerman/copyapp/releases/download/v1.0.2/use.mp4)

The website also includes these videos.

---

## For developers

Requirements: Go 1.27+ on Windows (PowerShell 7 for the scripts).

```
pwsh -File build.ps1 -Version 1.0.2 -Test   # vet + tests, then dist\copyapp.exe and dist\SHA256SUMS
pwsh -File docs\e2e\run.ps1                 # builds/tests a loopback-only exe in isolated temp folders
```

`build.ps1` embeds the icon, version information and manifest (`cmd/copyapp/winres/winres.json`, via go-winres), builds with `-H=windowsgui -s -w -X main.version=…` and prints the SHA-256. A plain `go build ./cmd/copyapp` also works (it uses the committed `rsrc_windows_amd64.syso`, version "dev").

Automated integration tests and recordings use `build.ps1 -LoopbackTest`, which writes `output/test-bin/copyapp.exe` with the `copyapp_test_loopback` build tag. Both listeners stay on this PC, including after self-install/update, so temporary copies do not repeatedly request LAN firewall access. Harnesses inspect build metadata before launch and refuse release binaries (including `-BaselineExe`). This exercises app behavior locally; a physical phone/LAN check still needs the normal release build. The test build never replaces `dist/copyapp.exe` or published downloads.

Changes: [CHANGELOG.md](CHANGELOG.md). License: [MIT](LICENSE).

`docs/e2e/browser.js` exercises real Chromium against an already-running isolated app using `COPYAPP_DATA_DIR`, `COPYAPP_RECEIVE_DIR` and Playwright/jsQR/pngjs on `NODE_PATH`. It changes the test clipboard and writes test received files; it never starts the executable. Native Android installation/Share behavior needs a real-device check. CI always tests/builds and uploads a workflow artifact; an existing published release is preserved without replacing its assets.
