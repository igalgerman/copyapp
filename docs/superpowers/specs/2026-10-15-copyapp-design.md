# CopyApp – Design Spec (2026-10-15)

## Goal
Copy/paste text, images and files between a Windows PC and an Android phone on the same Wi-Fi/LAN.
Simple, lightweight, little code, trivial install, good security, seamless as possible.

## Decisions
- Network: same LAN only. No cloud, no relay, no accounts.
- Architecture: one Go exe on the PC + a PWA on the phone (served by the exe). No APK.
- One phone paired to one PC (personal use). Manual/explicit transfers, no silent mirroring.
- Out of scope (YAGNI): internet/relay access, multiple phones, background clipboard mirroring, clipboard history, code signing.

## Components
### `copyapp.exe` (Go, single file, no runtime deps)
- HTTPS server on the LAN interface; embeds the PWA (HTML/JS/CSS/manifest/service worker) via `go:embed`.
- Windows clipboard read/write (text, image/PNG) via Win32 API.
- Saves received files to `%USERPROFILE%\Downloads\CopyApp`.
- Tray icon + menu: Show QR / Pair, Send clipboard to phone, Open received folder, Start with Windows (toggle), Re-pair, Quit.
- Global hotkey Ctrl+Alt+C: send current PC clipboard to phone. Explorer "Send to" shortcut: send a file to phone.
- Small drop target (tray window) for drag-and-drop file sending.
- Advertises `copyapp.local` via mDNS; fixed port, persisted self-signed certificate.
- PC toasts on receive ("Copied 'text…'" / "Saved file.pdf").

### Phone PWA
- Installable (manifest + service worker), Android share-target for text/images/files → POST `/send`.
- Buttons: Send clipboard (user gesture), pick file/photo.
- Inbox of items from the PC (SSE `/events`), each with Copy / Download; progress for large transfers; auto-reconnect with status banner.
- UI built by an Opus subagent using the frontend-design skill: mobile-first, big tap targets, light/dark, pairing empty-state, clear send/receive feedback.

## Data flow
- Phone → PC: `POST /send` (text JSON or multipart). PC sets clipboard (text/image) or saves file; toast shown. No PC interaction needed.
- PC → phone: item put in an in-memory outbox; phone receives via SSE `/events`, fetches `GET /item/<id>`; one tap copies (Android blocks background clipboard access – honest limit).
- Items kept in memory, size cap default 200 MB, expire ~10 min.

## Security
- TLS, self-signed cert generated on first run; SHA-256 fingerprint included in the QR payload/UI for verification.
- Pairing: QR holds `https://copyapp.local:PORT/pair#<256-bit token>` (also IP fallback). Token exchanged once for a session cookie (HttpOnly, Secure, SameSite=Strict). Pairing only open for 2 minutes after the user opens "Pair"; token single-use. "Re-pair" rotates secrets and invalidates the session.
- All endpoints require the session cookie. Reject non-private source IPs. Rate limiting, request size caps, sanitized filenames (no path traversal, no overwrite), no UPnP/inbound internet exposure.
- Windows firewall prompt (private networks) accepted once.

## Install / deploy
- `go build` → single `copyapp.exe` (~8 MB). Share directly or via GitHub Releases with a SHA-256 checksum. SmartScreen "unknown publisher" warning expected (unsigned).
- Phone: scan QR with camera → opens page and pairs → "Add to Home screen". Updates = replace the exe; PWA updates itself.

## Errors
- Connection lost → banner + auto-reconnect. Oversize/unsupported item → clear message on sender side. Port busy → next free port, persisted.

## Testing
- Go unit tests: auth/pairing, token expiry, filename sanitization, size limits, IP filter.
- End-to-end: headless browser (phone viewport) against the server.
- Manual checklist: real-phone pairing, share-sheet text/image/file, PC→phone, large file, reconnect after Wi-Fi drop.

## Build split
- Opus subagents: server/security/clipboard/tray, PWA UI/UX, implementation plan.
- Haiku subagents: boilerplate, README, test scaffolding.
