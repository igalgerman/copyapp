# Security

## Reporting a problem

Please report security problems privately through GitHub:
**[Report a vulnerability](https://github.com/igalgerman/copyapp/security/advisories/new)** (Security tab → "Report a vulnerability").
Do not open a public issue for them. Include the CopyApp version (bottom of the CopyApp window), Windows and Android versions, and the steps to reproduce. You should get an answer within a week.

Only the latest release is supported.

## Threat model in short

CopyApp moves clipboard items and files between **one Windows PC and one paired Android phone on the same local network**. The PC runs the local HTTPS server; no hosted relay, account or cloud is used for transfers.

Builds may optionally configure hosted Sentry diagnostics. Windows and the running phone page then send bounded, content-free action/stage/result events directly over outbound HTTPS. Clipboard contents, files, pairing tokens and credentials are excluded. Diagnostics are disabled without a DSN and do not relay transfers. See [configuration and limitations](docs/DIAGNOSTICS.md).

**What it protects against**

- **Other devices on the LAN.** Transfer, status and disconnect routes require the session cookie issued by scanning the pairing QR code. Static phone app files are public on the LAN. The token-exchange route is reachable without a session but requires the 256-bit, single-use pairing token, valid for 10 minutes. The public CA certificate is available while pairing is open, or to the paired phone. Sessions expire after 180 days without use; requests from public (non-private) source IP addresses are refused. Unpairing and replacing an existing phone clear the old pending outbox.
- **Web pages in your browsers.** Cookies are `HttpOnly`, `Secure` (phone) and `SameSite=Strict`; state-changing requests also check `Origin` / `Sec-Fetch-Site`. A share that reaches the installed phone app is only sent after the user taps **Send**.
- **Other Windows users and unrelated local web services.** PC control data, the QR code and settings require loopback requests addressed to `localhost`/`127.0.0.1` with explicit credentials. The tray opens the control window using a per-launch secret stored in `%APPDATA%\CopyApp\local.key` (owner-only ACL); its bootstrap URL is removed from the address bar. The page receives a separate random token in origin-scoped `sessionStorage` and sends it in `X-CopyApp-Session`. The server keeps these tokens in memory; they expire after 12 idle hours or an app restart. Localhost cookies do not authorize control access. A bare control page is a generic shell without private PC data. TLS keys are protected with Windows DPAPI. Programs running as your Windows user are outside this protection (see below).
- **Dangerous received files.** File names are sanitized (no paths, reserved names, bidi tricks). Saved files are marked with the Mark of the Web (`Zone.Identifier`, internet zone) where the filesystem supports alternate streams; shell-parsed types (`.lnk`, `.url`, `.scf`, `.library-ms`, `.searchConnector-ms`, …) get `.blocked` appended, and `desktop.ini` is renamed. Receive destinations are checked after resolving existing junctions, symbolic links, mapped drives and short paths. Network shares, device paths, Windows, Program Files and Start menu/Startup destinations are refused. If redirected Downloads is unsafe, the default is `%LOCALAPPDATA%\CopyApp\Received`; startup fails if neither default is safe. These checks do not make received executable files safe to run.
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
- Allowing CopyApp through the Windows firewall exposes the phone-facing HTTPS service to permitted private-source peers on that network, including the token-exchange endpoint while pairing is open. It does not expose the PC-only control window or its QR code to the LAN. Use trusted networks; the source-IP filter does not distinguish a trusted home network from an untrusted network using private addresses.
- Receive-folder validation cannot defend against same-user malware changing filesystem links after a check. Mark of the Web is unavailable on filesystems without alternate streams. Native Android certificate/PWA installation and Share-menu integration need device-specific verification.
