# Security

## Reporting a problem

Please report security problems privately through GitHub:
**[Report a vulnerability](https://github.com/igalgerman/copyapp/security/advisories/new)** (Security tab → "Report a vulnerability").
Do not open a public issue for them. Include the CopyApp version (bottom of the CopyApp window), Windows and Android versions, and the steps to reproduce. You should get an answer within a week.

Only the latest release is supported.

## Threat model in short

CopyApp moves clipboard items and files between **one Windows PC and one paired Android phone on the same local network**. There is no server, account or cloud.

**What it protects against**

- **Other devices on the LAN.** Every phone-facing route needs the session cookie the phone gets by scanning the pairing QR code. The pairing token is 256-bit, single-use and valid for 10 minutes; the session expires after 180 days without use. Requests from public (non-private) IP addresses are refused.
- **Web pages in your browsers.** Cookies are `HttpOnly`, `Secure` (phone) and `SameSite=Strict`; state-changing requests also check `Origin` / `Sec-Fetch-Site`. A share that reaches the installed phone app is only sent after the user taps **Send**.
- **Other Windows users and programs on the PC.** The PC-only pages (the CopyApp window, QR code, settings) accept only loopback requests addressed to `localhost`/`127.0.0.1` that carry a per-launch secret stored in `%APPDATA%\CopyApp\local.key` (owner-only ACL). The browser gets a separate random session token, kept in memory, never the secret itself. TLS keys are protected with Windows DPAPI.
- **Dangerous received files.** File names are sanitized (no paths, reserved names, bidi tricks). Every saved file gets the Mark of the Web (`Zone.Identifier`, internet zone), and shell-parsed types (`.lnk`, `.url`, `.scf`, `.library-ms`, `.searchConnector-ms`, `desktop.ini`, …) get `.blocked` appended. The save folder cannot be a network share, a device path, Windows, Program Files or a Start menu/Startup folder.
- **Leaks through the PC.** Toasts never show clipboard text. The log (`%APPDATA%\CopyApp\copyapp.log`, 1 MB, rotated) never contains clipboard content, tokens or keys; control characters are stripped. Clipboard content that a password manager marks as sensitive is not sent.
- **Noticing a pairing you did not make.** Each pairing shows a Windows notification with the phone's IP address, and the CopyApp window keeps showing it, with an **Unpair phone** button.

**What it does not protect against**

- **Someone who can intercept your Wi-Fi traffic while you pair.** By default the phone uses CopyApp after you tap through Chrome's certificate warning, so an attacker in the middle at that moment could pair instead of you. The certificate code in the app shows which PC you reached, but it cannot detect such an attacker. Pair only on your own trusted Wi-Fi, never on public Wi-Fi. Installing the CopyApp certificate (optional, "Install as an app") removes the warning; compare the certificate code on the PC and the phone before installing it.
- **Malware running as your Windows user.** It can read the clipboard anyway and can use DPAPI like CopyApp does.
- **A lost or stolen phone that stays on your Wi-Fi.** It keeps access until you choose **Unpair phone** (or 180 days pass).
- **Denial of service by the paired phone** beyond simple caps (2 concurrent uploads, 8 event streams, per-item size limit).

## The certificate authority

On first start CopyApp creates its own small certificate authority (CA) on the PC. It signs the HTTPS certificate for the PC's LAN address and is renewed when the address changes.

- The CA is **name-constrained**: it can only vouch for `copyapp.local`, `localhost`, loopback, link-local and private-network IP addresses (RFC 1918 and `fc00::/7`). It cannot be used for public web sites.
- Its private key never leaves the PC and is stored DPAPI-protected in `%APPDATA%\CopyApp`.
- Installing it on the phone is optional. You can remove it any time in Android settings (Trusted credentials → User).
- Known limit: the constraint covers all private IP ranges, not only your PC. If malware stole the CA key from your PC, it could impersonate other private-address HTTPS services (a router, a NAS) to a phone that installed the CA. Narrowing it to one address would break pairing whenever the PC's address changes, so it was kept; remove the certificate from the phone if you stop using CopyApp.

## Other known limits

- The executable is not code-signed, so Windows SmartScreen warns on first run. Check the download against `SHA256SUMS` from the GitHub release.
- Running any `copyapp.exe` replaces the installed copy (same user only; no version check).
- The Windows firewall may ask to allow CopyApp on public networks; that exposes the pairing page to other devices on such a network. Prefer marking your home Wi-Fi as a private network.
