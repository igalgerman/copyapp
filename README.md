# CopyApp

**[עברית](README.he.md) · [Русский](README.ru.md) · English · [Website](https://igalgerman.github.io/copyapp/)**

CopyApp lets you copy and paste between your Windows PC and your Android phone.

- Copy and paste **text**, **images**, and **files** between your PC and your phone.
- Works over **your own Wi-Fi**. Nothing is sent to the internet or to any cloud.
- It is **free**.

---

## Download

**Download `copyapp.exe` from the CopyApp website:**

https://copyapp.igalg.chatgpt.site/

---

## Install (3 steps)

> [!IMPORTANT]
> **Before you start**
> - Connect your phone to the **same Wi-Fi as your PC**. The pairing page shows the PC's Wi-Fi name.
> - A **guest Wi-Fi** or **"AP/client isolation"** on the router blocks the connection. Use the main Wi-Fi.
> - Your browser will say **"Your connection is not private"**. That's expected: tap **Advanced → Proceed** (this is your own PC on your own Wi-Fi).

1. **Download and run `copyapp.exe`** on your PC. CopyApp installs for your Windows user and creates Desktop and Start Menu shortcuts. Choose Yes or No when asked to start with Windows. The control window opens with status, settings and a pairing QR code; the tray icon stays near the clock.
2. **Scan the QR code** with your phone's camera and open the link.
3. **If the browser says the connection is not private, tap Advanced → Proceed.** This is expected: the PC is on your own Wi-Fi.

That's it. CopyApp is now open in your phone's browser, ready to send and receive. Tip: bookmark the page or choose **Add to Home screen** in Chrome's ⋮ menu.

> The first time only, Windows may show **"Windows protected your PC"** (click **More info → Run anyway**) and a **firewall prompt** (click **Allow**, and tick **private and public** networks). Both are one-time.

### Optional: make it a real app (about 1 minute)

Want CopyApp in Android's **Share** menu and no "Not secure" warning? On the phone, open **⋯ → Make it a real app** and follow the steps: on the PC choose **Pair phone** in the tray and open **Certificate code**, then check that both codes match, download the certificate, install it once in Android Settings, then choose **Install app** in Chrome's ⋮ menu.

---
## How to use it

**Phone to PC**
- Choose **Auto**, **Clipboard** or **File**, then tap **Send clipboard**, **Photo** or **File**. Auto follows your saved PC destination; Clipboard handles text and PNG images; File saves to the chosen folder. (If the browser won't share the clipboard, a text box opens: paste there and tap Send.)
- After the optional upgrade: use the Android **Share** menu on a text, image, or file, and choose **CopyApp**.

**PC to phone**
- Press **Ctrl+Alt+C** to send what is on your PC clipboard to your phone.
- Or use **Send clipboard to phone** in the tray menu. If Ctrl+Alt+C is unavailable, the PC shows the fallback **Ctrl+Alt+Shift+C**.
- Or right-click a file, choose **Send to**, then **Phone (CopyApp)**.

---

Open the **CopyApp Desktop shortcut** to change the receive destination, folder, file limit, language, or Start with Windows. Phone Settings can remember automatic text copying and file downloads. To remove CopyApp, choose **Uninstall** in the PC control window.

## Good to know (limits)

- Android only lets an app read the clipboard when you tap. So on the phone, you tap to copy an item.
- It only works when your PC and phone are on the **same Wi-Fi**.
- It works with **one phone** paired to one PC.
- Items are kept for **10 minutes**.
- The default limit is **2 GiB per file**. Change it in the PC control window; large files stream to disk.
- The app speaks **Hebrew** (default), **Russian** and **English**. Switch with **עב / РУ / EN** at the top of the phone screen, and with **שפה / Язык / Language** in the tray menu.

---

## Security

- All data sent between your PC and phone is **encrypted**.
- The pairing QR code can be used **only once** and works for **10 minutes**.
- CopyApp only talks on **your local network**. It does not open your PC to the internet.
- Received files are saved to **Downloads\CopyApp** by default. Choose another folder in the PC control window.
- You can **unpair at any time**: right-click the tray icon and choose **Re-pair phone**.

---

## Troubleshooting

- **"Connection refused", "took too long to respond" or the page never opens?** Check *Before you start*: same Wi-Fi as the PC, not a guest network. Then make sure the Windows firewall allows CopyApp on **private and public** networks (Windows Security → Firewall & network protection → Allow an app through firewall → tick both boxes for CopyApp). If your Wi-Fi is marked **Public** in Windows, the public box is required. The pairing page on the PC has the same tips under **Can't open the page?**
- **Can't connect?** Make sure your PC and phone are on the **same Wi-Fi**.
- **Can't connect on a guest Wi-Fi?** Guest networks often block devices from talking to each other. Use your main Wi-Fi instead.
- **Stuck on "not private" with no Advanced button?** Open the link in **Chrome**.
- **Your PC's IP address changed?** Choose **Pair phone** in the tray menu and scan the QR code again.
- **Phone not working after a change?** Right-click the tray icon, choose **Re-pair phone**, and scan the new QR code.
- **Still failing?** Try turning **mobile data off** briefly (some phones send traffic over mobile data when the Wi-Fi shows no internet).

---

## Videos

- [Install video](https://copyapp.igalg.chatgpt.site/assets/install-1.0.0.mp4)
- [Use video](https://copyapp.igalg.chatgpt.site/assets/use-1.0.0.mp4)

The website also includes these videos.

---

## For developers

```
go build -ldflags "-H=windowsgui -s -w" -o dist\copyapp.exe ./cmd/copyapp
go test ./...
```

SHA-256: see [release notes](docs/RELEASE_NOTES.md)
