# CopyApp

**[עברית](README.he.md) · [Русский](README.ru.md) · English**

CopyApp lets you copy and paste between your Windows PC and your Android phone.

- Copy and paste **text**, **images**, and **files** between your PC and your phone.
- Works over **your own Wi-Fi**. Nothing is sent to the internet or to any cloud.
- It is **free**.

---

## Download

**Download `copyapp.exe` from the latest Release:**

https://github.com/igalgerman/copyapp/releases/latest

---

## Install (3 steps)

> [!IMPORTANT]
> **Before you start**
> - Connect your phone to the **same Wi-Fi as your PC**. The pairing page shows the PC's Wi-Fi name.
> - A **guest Wi-Fi** or **"AP/client isolation"** on the router blocks the connection. Use the main Wi-Fi.
> - Your browser will say **"Your connection is not private"**. That's expected: tap **Advanced → Proceed** (this is your own PC on your own Wi-Fi).

1. **Download and run `copyapp.exe`** on your PC. A pairing page with a QR code opens, and a CopyApp icon appears in the tray (near the clock).
2. **Scan the QR code** with your phone's camera and open the link.
3. **If the browser says the connection is not private, tap Advanced → Proceed.** This is expected: the PC is on your own Wi-Fi.

That's it. CopyApp is now open in your phone's browser, ready to send and receive. Tip: bookmark the page or choose **Add to Home screen** in Chrome's ⋮ menu.

> The first time only, Windows may show **"Windows protected your PC"** (click **More info → Run anyway**) and a **firewall prompt** (click **Allow**, and tick **private and public** networks). Both are one-time.

### Optional: make it a real app (about 1 minute)

Want CopyApp in Android's **Share** menu and no "Not secure" warning? On the phone, open **⋯ → Make it a real app** and follow the steps: check the code matches the one on your PC's pairing page, download the certificate, install it once in Android Settings, then choose **Install app** in Chrome's ⋮ menu.

---
## How to use it

**Phone to PC**
- Open CopyApp on your phone and tap **Send clipboard**, **Photo** or **File**. (If the browser won't share the clipboard, a text box opens: paste there and tap Send.)
- After the optional upgrade: use the Android **Share** menu on a text, image, or file, and choose **CopyApp**.

**PC to phone**
- Press **Ctrl+Alt+C** to send what is on your PC clipboard to your phone.
- Or right-click the CopyApp tray icon and choose **Send clipboard to phone**.
- Or right-click a file, choose **Send to**, then **Phone (CopyApp)**.

---

## Good to know (limits)

- Android only lets an app read the clipboard when you tap. So on the phone, you tap to copy an item.
- It only works when your PC and phone are on the **same Wi-Fi**.
- It works with **one phone** paired to one PC.
- Items are kept for **10 minutes**.
- The largest item you can send is **200 MB**.
- The app speaks **Hebrew** (default), **Russian** and **English**. Switch with **עב / РУ / EN** at the top of the phone screen, and with **שפה / Язык / Language** in the tray menu.

---

## Security

- All data sent between your PC and phone is **encrypted**.
- The pairing QR code can be used **only once** and works for **10 minutes**.
- CopyApp only talks on **your local network**. It does not open your PC to the internet.
- Received files are saved to **Downloads\CopyApp** on your PC.
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

- [Install video](docs/video/install.mp4)
- [Use video](docs/video/use.mp4)

These videos are also attached to the Release.

---

## For developers

```
go build -ldflags "-H=windowsgui -s -w" -o dist\copyapp.exe ./cmd/copyapp
go test ./...
```

SHA-256: (see Release notes)
