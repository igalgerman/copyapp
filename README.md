# CopyApp

CopyApp lets you copy and paste between your Windows PC and your Android phone.

- Copy and paste **text**, **images**, and **files** between your PC and your phone.
- Works over **your own Wi-Fi**. Nothing is sent to the internet or to any cloud.
- It is **free**.

---

## Download

**Download `copyapp.exe` from the latest Release:**

https://github.com/igalgerman/copyapp/releases/latest

---

## Install

1. **Run `copyapp.exe`** on your PC. A CopyApp icon appears in the tray (near the clock).
2. **Windows may show "Windows protected your PC".** This is normal for a new app. Click **More info**, then click **Run anyway**.
   - If you see a **firewall prompt**, click **Allow** on **private networks**.
3. **Scan the QR code** that appears on your PC with your phone's camera. This opens CopyApp on your phone and pairs it with your PC.
4. **On the phone**, follow the **4-step one-time certificate install** shown on the screen. Then tap **Add to Home screen**.

---

## How to use it

**Phone to PC**
- Use the Android **Share** menu on a text, image, or file, and choose **CopyApp**.
- Or open CopyApp on your phone and tap **Send clipboard**.

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

---

## Security

- All data sent between your PC and phone is **encrypted**.
- The pairing QR code can be used **only once** and works for **2 minutes**.
- CopyApp only talks on **your local network**. It does not open your PC to the internet.
- Received files are saved to **Downloads\CopyApp** on your PC.
- You can **unpair at any time**: right-click the tray icon and choose **Re-pair phone**.

---

## Troubleshooting

- **Can't connect?** Make sure your PC and phone are on the **same Wi-Fi**.
- **Can't connect on a guest Wi-Fi?** Guest networks often block devices from talking to each other. Use your main Wi-Fi instead.
- **Still can't connect?** Check that your **firewall** allows CopyApp on private networks.
- **Your PC's IP address changed?** You don't need to do anything. CopyApp keeps working.
- **Phone not working after a change?** Right-click the tray icon, choose **Re-pair phone**, and scan the new QR code.

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
