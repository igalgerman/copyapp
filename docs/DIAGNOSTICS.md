# Hosted diagnostics (issue #1)

Diagnostics are optional and disabled when no DSN is configured. LAN clipboard,
image and file transfers continue locally. No project has been provisioned or
live ingestion verified by this change.

## One-time owner setup

1. Create a hosted Sentry project. Copy its **public DSN**, not an auth/admin token.
   Use an HTTPS hosted ingestion DSN (`o….ingest[.us|.de].sentry.io`).
2. Open [repository Actions variables](https://github.com/igalgerman/copyapp/settings/variables/actions)
   (**Settings → Secrets and variables → Actions → Variables → New repository variable**).
   Set `COPYAPP_SENTRY_DSN` to the public DSN. The next release build embeds it in
   the executable; setting the variable alone does not modify installed apps. For a manual build use
   `./build.ps1 -Version 1.0.2 -SentryDSN 'https://PUBLICKEY@o123.ingest.sentry.io/456'`.
   Replace the example with the actual DSN; its public key is hexadecimal.
   A nonempty process-level `COPYAPP_SENTRY_DSN` overrides the embedded value.
   For a local smoke test, quit the existing tray app, set that environment
   variable in PowerShell, and start `CopyApp.exe` from the same window.
3. Publish a **new release version** and install its rebuilt Windows app. Existing
   release assets are deliberately preserved: rerunning an old tag does not
   replace its executable. Open the control window and find **Support
   ID**. Pair a phone; its authenticated `/status` response supplies that same PC
   support ID to the phone's direct diagnostic sender. Search Sentry Logs for
   `support_id` plus a time window; use `session_id`, `operation_id`, `action`,
   `stage` and `result` to narrow the timeline. Share/export only the selected
   diagnostic events with the debugging agent. A healthy activation shows a
   Windows `app.launch.ok` log and, after opening the PWA, an Android
   `app.launch.ok` log. No received events means activation is still unverified.

No Sentry administrator credentials belong in the app. A DSN is public ingestion
configuration. Windows and the phone each contact Sentry directly over HTTPS;
phone diagnostics are never relayed by Windows. The public DSN/version and local SDK
are cached after a successful load so a previously loaded page can report while
the PC is unreachable. The first successful page/config load still requires it. The public LAN
`/diagnostics/config` endpoint exposes only the DSN and app version, enabling
pre-pairing page errors once the phone has loaded the app successfully.

## Build

Install Go matching `go.mod` and Node 22.12+ on the build machine. Run:

```
npm ci --ignore-scripts
npm run build:diagnostics
npm run test:diagnostics
go mod download
go test -mod=readonly ./...
```

`build.ps1`, release CI and validation CI build the browser SDK before embedding
`internal/server/web`. `scripts/diagnostics-entry.mjs` bundles the official
`@sentry/browser` 11.8.0 with esbuild; no CDN script is loaded at runtime.
The npm and Go dependency manifests are checked in. `npm ci` uses the npm lock,
and Go tests/builds use `-mod=readonly`; dependency updates are deliberate source
changes. The generated SDK bundle is excluded from Git. Direct `go build` requires the
bundle step first for browser diagnostics. Go uses official `sentry-go` v0.50.0.
Its source API was checked at that tag: logs are enabled automatically (the older
`EnableLogs` option no longer exists). Source maps and tracing are not enabled.

## Three-step device verification

1. On real Windows and Android, pair and send text both directions. Find the
   same `support_id` in Sentry for both sides. A phone upload includes a random
   `operation_id` also logged by the PC on `/send`; compare `start`, HTTP result,
   and display/clipboard stages. A PC publish/queue outcome is not proof of a
   phone clipboard write. Phone `display` and `automatic_write` are distinct.
2. Refuse browser clipboard access, disconnect the phone's Wi-Fi, try an expired
   pairing QR and exercise the Windows hotkey. Check stage/result markers rather
   than interpreting a generic request success as delivery. An app exception
   emits a sanitized crash marker; detailed local panic output is not uploaded.
3. Block Sentry ingestion while keeping LAN connectivity. Repeat transfers and
   confirm they remain usable. Reopen/reload the app; Support ID persists,
   session ID changes. Confirm local `%APPDATA%\CopyApp\copyapp.log` still rotates.

## Bounds and privacy

- Only explicit fixed stage codes are forwarded; the general rotating log is
  retained locally and is not streamed to Sentry. Clipboard text, file data and
  names, raw error messages, auth cookies, QR tokens, request bodies and URLs
  are excluded. Browser automatic integrations (console/network breadcrumbs,
  replay, DOM capture and tracing) are disabled.
- Each side caps remote stage emission at 60/minute. Windows uses the SDK's
  bounded 1000-log ring buffer, a 3-second HTTP timeout and a 1-second shutdown
  flush. The browser keeps at most 16 pre-initialization stages, caps in-flight
  SDK requests at eight and aborts ingestion requests after three seconds.
  The SDK honors server rate limits; there is no persistent offline queue.
  Offline and overflow stages may be dropped. Local Windows stages continue.
- Phone IDs use origin-local browser storage; re-pairing at another PC IP or
  clearing browser storage can create another ID. The PC Support ID bridges
  authenticated timelines. It is not an authentication credential.
- The PWA reports only while its page executes. It cannot diagnose a page that
  never loads, an OS-killed browser, or every suspended background failure.
  Native companion telemetry is separate from the PWA; the companion's server
  request outcomes appear on Windows, but native pre-connection failure logging
  needs device verification/instrumentation.
- Neither SDK outages nor exceptions thrown by telemetry are allowed to break
  clipboard actions. CSP allows only the exact validated HTTPS ingestion origin
  in addition to `'self'`; scripts remain local.

## Outstanding acceptance and quotas

Real Sentry ingestion, two-device failure timelines, Windows crash behavior and
physical-device/offline resource measurements remain unverified. This workspace
has no configured DSN. Windows CI resolved and bundled the official SDKs, passed
`go vet ./...`, `go test -count=1 ./...` and built the executable in
[PR #5](https://github.com/igalgerman/copyapp/pull/5). The four JavaScript
privacy/rate/offline checks and existing phone/service-worker checks also pass.

Do not assume that a free project is provisioned or that today's allowance is
fixed. Before enabling releases, check the current Developer plan and your
project's Logs/error ingestion, retention, overage and rate-limit settings at
<https://sentry.io/pricing/> and in Usage Stats. Current live free-plan quotas
could not be verified here; the application cap is a ceiling, not an account
quota guarantee. Reduce event volume or clear the repository DSN variable and build a new release
if the account budget requires it. An empty runtime variable does not override a
DSN already embedded in an installed executable.
