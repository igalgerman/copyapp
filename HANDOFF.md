# CopyApp handoff — 2026-10-20

## Immediate objective and user preferences

The user wants the agent to finish Sentry setup for CopyApp, rather than repeatedly sending them manual instructions. They have now requested this handoff for another agent because installer/network attempts made no progress.

Communicate in Hebrew, keep updates concise, and do not leave the user waiting silently. Do not repeat long permission/install attempts without a result. The user previously authorized parallel agents, finishing the code, merging the PR, and updating the website. They specifically want BOTH the Android APK and the browser/PWA supported.

## Verified GitHub state

- Account/repository: `igalgerman/copyapp`, default branch `master`.
- PR: https://github.com/igalgerman/copyapp/pull/5 — **merged**, not a pending draft.
- Squash merge commit: `c728bef33b3f6f857a1806e114c4a61220faff9f`.
- PR head before merge: `ead836675c3095caf1f32cd5be68d3b3acd43d5a`.
- Issues #1–#4 remain open because their live-service/physical-device/video acceptance is incomplete. Do not claim they are finished or automatically close them.

Verified successful runs:

| Check | URL |
| --- | --- |
| Windows vet, all Go tests, EXE build, browser bundle, site checks, locks/format | https://github.com/igalgerman/copyapp/actions/runs/39417174317 |
| Android debug build/lint, unsigned-release refusal, temporary-key signed release build/lint/signature check and cleanup | https://github.com/igalgerman/copyapp/actions/runs/39417174321 |
| Post-merge Android build/signing checks | https://github.com/igalgerman/copyapp/actions/runs/39418471861 |
| Post-merge website deployment | https://github.com/igalgerman/copyapp/actions/runs/39418471792 |

## Local workspace caveat — important

Code is at `/workspace/copyapp`. Its working files contain the merged implementation, but its local Git history is **synthetic and stale**: `HEAD` is `41a99ad`, a baseline snapshot of upstream `fff066f8c663ea28e217c3b4b13412b3b46e57ad`. Many implemented files therefore appear modified/untracked even though they were published and merged through GitHub connector tools.

Do not reset/clean this directory or mistake its status for an unmerged PR. Prefer a fresh clone of current `master` into a new directory if networking works. GitHub is authoritative. `origin` is https://github.com/igalgerman/copyapp.git. A full environment-specific copy was originally saved at `/workspace/HANDOFF.md`; this repository copy omits the screenshot attachment path and the opaque Sites project identifier.

## What is implemented

- Optional native Kotlin companion: text/URL clipboard sending via a Quick Settings tile and a small focused Activity. Clipboard is read only after focus. Repeated taps/rotation/process restoration avoid overlapping or silent duplicate sends.
- PWA remains the full text/image/file route in both directions. APK is additive, not a replacement. Images/files and PC-to-phone receipt still use the PWA.
- Separate persisted companion/browser credentials preserve both sessions. Windows Unpair revokes both. Native pairing verifies the public CA fingerprint and keeps standard TLS hostname verification.
- Windows hotkey registration/closed-channel recovery, both event streams drained, bounded clipboard retries, fail-closed sensitivity checks, actual shortcut and last-attempt status. Queued/published is explicitly NOT proof of Android receipt.
- Sentry Go/browser integration, stable Support ID and session/operation fields, privacy allowlists, bounded queues/rates, rotating local log retained, narrow ingestion CSP, locally bundled browser SDK and cached browser configuration.
- Stable Android release-signing/publishing workflow is implemented. PR builds use no owner secrets. CI tests signing with an ephemeral key and deletes that key and test release APK; only debug APK is a development artifact.
- Updated trilingual website, verified dependency locks and build checks, real-device Hebrew tutorial recording/assembly kit.

## Sentry — status (updated 2026-10-20)

**Done:**

- Sentry plugin MCP authenticated via OAuth (the npx installer / run code is no longer needed; do not use it).
- Created Sentry project **`copyapp`** (platform `go`) in org **`igorgerman`** (team `igorgerman`, project ID 4512288841793536). One project serves both Windows (Go) and the PWA, since the app embeds a single DSN and serves it to the browser.
- Public DSN: `https://bfa01406d23cd54e76c4a0305e4e4755@o4512288490520576.ingest.us.sentry.io/4512288841793536` (public ingestion key, safe to store; passes `build.ps1` validation).
- Set GitHub Actions repository variable `COPYAPP_SENTRY_DSN` on `igalgerman/copyapp` via authenticated `gh` (verified with `gh variable list`).
- Ran a temporary Go test (deleted afterwards, nothing committed) that called `diagnostics.Start` + `Emit` with the DSN. It passed and flushed; Support ID in that run was `522952e8949aeaff6fc04a808d1d10b5`.

**NOT yet verified / remaining:**

1. ~~Live ingestion~~ **Confirmed** (2026-10-20): `app.launch.ok` and `setup.verify.ok` from the test run appeared in Sentry Logs for project `copyapp` after a short ingestion delay.
2. A `build.yml` workflow_dispatch run on `master` was started 2026-10-20 (run 39433477886); download its `copyapp-0.0.0` artifact, close the old CopyApp and run the new EXE; the old installed/published v1.0.2 EXE predates the integration and will not report.
3. Verify real Windows + phone events by `support_id`, a deliberate safe failure, and offline behavior (see `docs/DIAGNOSTICS.md`, three-step verification).
4. Optional: add a Sentry alert once events flow.

Activation details:

- Docs: `docs/DIAGNOSTICS.md`.
- Code: `internal/diagnostics/`, `internal/server/web/diagnostics.js`, `scripts/diagnostics-entry.mjs`.
- Public DSN can be embedded using `build.ps1 -SentryDSN ...` or the release workflow repository variable. A process environment `COPYAPP_SENTRY_DSN` overrides the embedded value.
- Build workflow: https://github.com/igalgerman/copyapp/actions/workflows/build.yml supports manual dispatch on `master`; this produces `copyapp-0.0.0` artifact with EXE/checksum, without publishing a stable release. The user must close the old CopyApp and run the newly built EXE for that DSN to take effect.
- Existing published v1.0.2 EXE predates the new integration. Do not imply it uploads new diagnostics.
- Local logs: `%APPDATA%\CopyApp\copyapp.log`.
- SDKs are pinned/compiled: `sentry-go v0.50.0`, `@sentry/browser 11.8.0`. The Go SDK no longer has the old `EnableLogs` option. Its Event schema has no `Extra`; current sanitizer/tests already account for this.
- Browser telemetry works while its page executes; native companion pre-connection/focus failures currently show UI errors but do NOT upload directly to Sentry. Windows logs native request outcomes. Do not overstate Android telemetry coverage.

## Why previous installer attempts failed

- Default `curl` to the npm registry repeatedly failed immediately: `Failed to connect to proxy port 8080 ... Could not connect to server`.
- The managed environment reported an enforced **unrestricted** HTTP policy, but that did not imply shell socket access. A local Python HTTP server failed with `PermissionError: Operation not permitted`; Chromium also failed on socket setup.
- Two attempts using `exec_command` with additional network permission never returned execution output and were ultimately aborted by the user after long waits. There was NO reported automatic-approval rejection; do not invent one or claim the installer ran.
- The last command requested was bounded `timeout 35s ... npx --yes @sentry/agent-plugin --help`, not project creation. Waiting at a permission boundary still lasted much longer than the command timeout. Avoid repeating that loop.
- Last verified `/tmp/copyapp-sentry-npm-cache` did not exist and no cached `@sentry/agent-plugin` package was present. No installation is progressing in the background.
- No useful Go compiler/gofmt/Gradle/Android SDK was available locally. `/usr/bin/go` is an unrelated command, not the Go toolchain. Node/npm/npx, Python, gh, Chromium and JDK keytool binaries exist. Compilation/testing was done through GitHub Actions.

## Website — distinguish the two URLs

- **Updated and successfully deployed:** https://igalgerman.github.io/copyapp/
- **Original Sites URL, NOT updated by this work:** https://copyapp.igalg.chatgpt.site/
- Reviewed source is `site/`; `.github/workflows/pages.yml` deploys it on relevant pushes to `master`.
- Site displays supported v1.0.2 PWA/browser downloads separately from the experimental APK. It explains that the old v1.0.2 Windows download cannot pair with the APK; matching new Windows/Android development builds are required.
- Existing v1.0.2 videos remain linked and are labeled as browser walkthroughs with some illustrated OS scenes, not the new real-device tutorials.
- Tests: `npm run test:site` (138 translation keys in English/Russian, 37 unique IDs, anchors and availability links).

Original Sites project:

- Discover the existing CopyApp project by its exact public URL using Sites tools; reuse the returned opaque project ID.
- Audience: **public**, preserve it.
- Last inspected saved version number: 10.
- Native get_site/list_sites/credential calls worked, but required Sites source helper `site-workflow.mjs` was absent from the filesystem and could not be read through skill resources. Shell networking also failed.
- A short-lived source credential was obtained in previous session memory only; it is not in this file and should be treated as expired. Obtain a fresh credential if needed.
- Source provider: `cloudflare_artifact`, branch `main`, not the user's GitHub repository.
- Follow the current Sites skill to open/sync source, package and publish the exact pushed state. Do not upload a modified archive against an old unrelated source commit or claim GitHub Pages deployment updated the original Sites URL.

## Android stable release — still requires owner configuration

See `docs/ANDROID.md`, `android/create-signing-key.sh`, `android/app/build.gradle.kts` and `.github/workflows/android.yml`.

No owner release key was generated or uploaded. Required Actions secrets:

- `COPYAPP_ANDROID_KEYSTORE_BASE64`
- `COPYAPP_ANDROID_KEYSTORE_PASSWORD`
- `COPYAPP_ANDROID_KEY_ALIAS`
- `COPYAPP_ANDROID_KEY_PASSWORD`

Required public repository variable: `COPYAPP_ANDROID_CERT_SHA256`.

The helper creates an owner key interactively OUTSIDE the repository and refuses overwrite. Back it up privately. Never commit a keystore/password. The workflow verifies the expected signing certificate, creates APK/checksums, waits for the matching Windows release, and never overwrites differing published assets. Do not publish the CI throwaway-key APK as a real release.

Development downloads:

- https://github.com/igalgerman/copyapp/actions/workflows/android.yml — `CopyApp-Android-development`.
- https://github.com/igalgerman/copyapp/actions/workflows/validate.yml — `CopyApp-Windows-development`.
- GitHub login/ZIP extraction may be required. Debug signing keys can differ between builds, requiring reinstall/pairing. No stable signed APK release was published.

## Remaining physical acceptance and videos

- Real Windows hotkey diagnosis/20 separate presses across Hebrew/English layouts, conflicts, reconnect and clipboard-lock checks remain unperformed. The original intermittent symptom is NOT proven resolved. The third-party hotkey SDK can internally exit its pump without closing public channels; source recovery cannot observe every such failure.
- Real Samsung/Android tile focus, QR readability, TLS pairing, PWA coexistence, revocation and upgrade tests remain unperformed. CI is not physical-device validation.
- Four new real Hebrew tutorial MP4s are NOT recorded or published. Kit: `docs/video/REAL-CAPTURE.he.md`, `docs/video/real-capture.example.json`, `docs/video/assemble-real.py`.
- The assembler was tested with clearly synthetic temporary tooling fixtures only. Do not present those as genuine tutorials. Capture real Windows/Android source and destination behavior, then verify WhatsApp playback and upload final MP4s as release assets.

## Useful local checks

```sh
cd /workspace/copyapp
npm run test:site
npm run test:diagnostics
node internal/server/testdata/i18n_test.mjs internal/server/web/i18n.js internal/server/web/index.html internal/server/web/app.js
node internal/server/testdata/phone_test.mjs internal/server/web/app.js
node internal/server/testdata/sw_test.mjs internal/server/web/sw.js
bash -n android/create-signing-key.sh
git diff --check
```

Use current GitHub Actions for Windows/Android compiled checks unless the new environment supplies real toolchains. All prior successful builds and deployments are linked above; preserve their distinction from account provisioning and real-device QA.
