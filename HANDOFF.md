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

## Sentry — current blocker and next steps

**No Sentry project/DSN has been connected, no live ingestion has been verified, and diagnostics remain disabled without configuration.** The user originally said there was no Sentry project, then created/accessed an account and shared the agent-onboarding screen.

The screenshot shows organization **`igorgerman`** (different spelling from GitHub `igalgerman`), at `igorgerman.sentry.io`, offering the official command pattern:

```sh
npx @sentry/agent-plugin install '<org-slug>#<fresh-run-code>'
```

Do NOT reuse an old onboarding run code. No codes/tokens are included here. The screenshot's code may have expired; obtain a fresh one only if the supported flow needs it. The installer was never successfully downloaded. No plugin installation or Sentry account connection was established by this agent.

Suggested sequence for the next agent:

1. Establish that the execution environment actually has working network access; inspect the current environment/runtime instructions and preserve its proxy/CA configuration. Use a short bounded check first.
2. Check available Sentry tools/plugin capabilities again. Previous tool discovery found no Sentry tools, and plugin search returned no Sentry plugin. The official npm installer is the path the user explicitly asked us to use.
3. Run the official installer/help in a working environment with the required runtime permissions; keep output polling short and report progress. Installing a plugin does not automatically mean its tools are loaded into the current agent session.
4. Through the authenticated supported flow, create/select the CopyApp Sentry project and obtain its public DSN. Never ask the user to paste an admin token into chat or commit credentials.
5. Set GitHub Actions **repository variable** `COPYAPP_SENTRY_DSN` to the public DSN. Native GitHub connector tools available in this session could not manage repository variables/secrets. `gh` exists locally, but its authentication has NOT been established/verified. Use supported authenticated tooling; do not assume the GitHub connector token is a shell credential.
6. Build a new Windows executable with the DSN and install/run that build. Updating the website or repository variable alone does not update an existing installed EXE.
7. Verify real events in Sentry Logs by `support_id`, including a deliberate safe failure and offline behavior. Do not call the setup complete before receiving events.

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
