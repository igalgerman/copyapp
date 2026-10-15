# CopyApp

**[עברית](README.he.md) · [Русский](README.ru.md) · English · [Website](https://igalgerman.github.io/copyapp/)**

CopyApp lets you copy and paste between your Windows PC and your Android phone.

- Copy and paste **text**, **images**, and **files** between your PC and your phone.
- Works over **your own Wi-Fi**. Nothing is sent to the internet or to any cloud.
- It is **free** and open source ([MIT license](LICENSE)).

---

## Download

**Download `copyapp.exe` from the CopyApp website:**

https://copyapp.igalg.chatgpt.site/

or from the [latest GitHub release](https://github.com/igalgerman/copyapp/releases/latest) (with a `SHA256SUMS` file to check the download).

---

## Install (3 steps)

> [!IMPORTANT]
> **Before you start**
> - Connect your phone to the **same Wi-Fi as your PC**. The CopyApp window shows the PC's Wi-Fi name.
> - A **guest Wi-Fi** or **"AP/client isolation"** on the router blocks the connection. Use the main Wi-Fi.
> - Pair only on **your own Wi-Fi** at home or at work, never on public Wi-Fi in a café or hotel.

1. **Download and run `copyapp.exe`** on your PC. CopyApp installs itself for your Windows user (no administrator rights needed) and creates Desktop and Start menu shortcuts. Answer **Yes** or **No** to the question about starting with Windows. The CopyApp window opens with a QR code; the CopyApp icon stays near the clock (it may be hidden under the **^** arrow).
2. **Scan the QR code** with your phone's camera and open the link.
3. **If the browser says "Your connection is not private", tap Advanced → Proceed.** That's fine here: it is your own PC on your own Wi-Fi.

That's it. CopyApp is now open in your phone's browser, ready to send and receive. Tip: bookmark the page or choose **Add to Home screen** in Chrome's ⋮ menu.

> The first time only, Windows may show **"Windows protected your PC"** (click **More info → Run anyway**: CopyApp is not signed with a paid certificate) and a **firewall prompt** (click **Allow**). Both are one-time.

### Optional: install as an app (about 1 minute)

Want CopyApp in Android's **Share** menu and no "Not secure" warning? On the phone, open **⋯ → Install as an app** and follow the steps there:

1. On the PC, open the CopyApp window and expand **Certificate code**.
2. Check that the code on the phone is the same.
3. Download the certificate and install it once in Android settings (menu names differ a little between phone makers).
4. In Chrome's ⋮ menu choose **Install app**.

The certificate can only vouch for addresses on your local network, and you can remove it any time in Android settings.

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
- Press **Ctrl+Alt+C** to send what is on your PC clipboard to your phone. A green **✓ Sent to phone** box appears near the clock for a moment, and you hear a short sound.
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
- The app is available in **Hebrew** (default), **Russian** and **English**. Switch with **עב / РУ / EN** at the top of the phone screen and of the CopyApp window.

---

## Security

- All data sent between your PC and phone is **encrypted**.
- The pairing QR code can be used **only once** and works for **10 minutes**.
- When a phone pairs, the PC shows a notification with the phone's address, and the CopyApp window keeps showing it. If it was not you, choose **Unpair phone**.
- CopyApp only talks on **your local network**. It does not open your PC to the internet.
- Received files are saved to **Downloads\CopyApp** by default and are marked as "downloaded from the internet", so Windows warns before opening programs or Office files from them.
- You can **unpair at any time**: in the CopyApp window or the icon menu choose **Unpair phone**; on the phone choose **⋯ → Unpair this phone**.
- A phone that is not used for 180 days must pair again.

More details: [SECURITY.md](SECURITY.md).

---

## Troubleshooting

- **"Refused to connect", "took too long to respond" or the page never opens?** Check *Before you start*: same Wi-Fi as the PC, not a guest network. Then make sure the Windows firewall allows CopyApp (Windows Security → Firewall & network protection → Allow an app through firewall → tick CopyApp). If your Wi-Fi is marked **Public** in Windows, either switch it to **Private** (Settings → Network & internet → Wi-Fi → your network) or tick **Public** for CopyApp. The CopyApp window has the same tips under **Can't open the page?**
- **Pressed Ctrl+Alt+C and nothing happened?**
  - You should hear a sound and see a green **✓ Sent to phone** box near the clock. No sound and no box: another program may have taken the shortcut; the CopyApp window shows the active shortcut under **Send the PC clipboard to the phone**, and its **Send PC clipboard to phone now** button always works.
  - The box appeared but the phone shows nothing: keep CopyApp open on the phone (the dot at the top says **Connected**).
  - The log file has one line per attempt (never the clipboard content): `%APPDATA%\CopyApp\copyapp.log`. Its path is also shown at the bottom of the CopyApp window.
- **Stuck on "not private" with no Advanced button?** Open the link in **Chrome**.
- **Your PC's IP address changed?** Choose **Pair phone** in the icon menu and scan the QR code again. Tip: reserve the PC's address in your router.
- **Still failing?** Try turning **mobile data off** briefly (some phones send traffic over mobile data when the Wi-Fi shows no internet).

---

## Videos

- [Install video](https://copyapp.igalg.chatgpt.site/assets/install-1.0.0.mp4)
- [Use video](https://copyapp.igalg.chatgpt.site/assets/use-1.0.0.mp4)

The website also includes these videos.

---

## For developers

Requirements: Go 1.27+ on Windows (PowerShell 7 for the scripts).

```
pwsh -File build.ps1 -Version 1.0.0 -Test   # vet + tests, then dist\copyapp.exe and dist\SHA256SUMS
pwsh -File docs\e2e\run.ps1                 # end-to-end test of the real exe (isolated temp folders)
```

`build.ps1` embeds the icon, version information and manifest (`cmd/copyapp/winres/winres.json`, via go-winres), builds with `-H=windowsgui -s -w -X main.version=…` and prints the SHA-256. A plain `go build ./cmd/copyapp` also works (it uses the committed `rsrc_windows_amd64.syso`, version "dev").

Changes: [CHANGELOG.md](CHANGELOG.md). License: [MIT](LICENSE).
