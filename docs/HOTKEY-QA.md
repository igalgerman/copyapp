# Windows clipboard shortcut: implementation and release gate

## What was established by inspection

`golang.design/x/hotkey v0.6.4` registers on its own locked OS thread. Calling
Register from a goroutine is supported on Windows; moving registration to the
tray thread is not justified. Its event queues have unbounded capacity. CopyApp
previously consumed only Keydown, leaving Keyup queued indefinitely. It also
returned permanently if the down channel closed, and never retried registration
when both accelerators were unavailable.

The clipboard library returns ErrUnavailable when the clipboard remains locked
past its own retry window. CopyApp previously discarded that error and reported
an empty clipboard. It also treated unknown sensitivity as permission to send.
The new read path preserves errors, fails closed on unknown sensitivity, retries
transient availability errors at most three times under a 1.5 second context,
and gives a distinct actionable read failure. Ordinary empty data is not retried.

The shortcut listener now drains both streams and
retries registration with exponential delay capped at 30 seconds, and recovers
from a closed stream after cleanup. The tray/control window show the registered
accelerator or an unavailable warning, never a guessed default. Cleanup has a
two-second deadline; a stuck library unregister produces a restart warning and
leaves manual send usable rather than spawning repeated stuck goroutines.

**This does not establish the cause of the owner's intermittent failures.**
In particular, the library's internal Windows pump can return on WM_QUIT or
GetMessage failure without closing its public streams. CopyApp cannot detect that
through this SDK's public channel API. Physical diagnosis is required before
replacing the library; no keyboard hook was added. No missing key is inferred
from the absence of logs. The deferred receiver in the original implementation
kept the hotkey alive, so there is no demonstrated finalizer/GC bug.

## Status meanings

- `queued`: saved in the PC outbox, no SSE connection observed after publication.
- `published`: saved in the outbox and an SSE connection was observed; **not an
  acknowledgement of display, download or Android clipboard access**.
- `readFailed`, `empty`, `secret`, `busy`, `failed`: distinct failure/skip stages.

The control window shows the last completed attempt, trigger and time in its
state API; its visible status includes result and time. The existing manual send
button and tray action share the same path. Local logs include a numbered
attempt, source, result and elapsed time; Sentry uses a random operation ID.

## Required physical Windows + Android verification (not performed here)

1. Pair a real phone. Open the PC control window, confirm the active accelerator,
   and keep the phone PWA visible. Copy unique numbered Hebrew and English text.
   Press and release the displayed accelerator 20 separate times per layout,
   observing each item on the phone once. Hold a key to record Windows autorepeat
   behavior; repeats are not inferred/suppressed from separately queued events.
   Repeat with Chrome and VS Code focused, minimized/in tray, after reboot and
   autostart, through RDP, and with an AltGr layout. Record release, Windows build,
   keyboard layout, registered accelerator, attempt numbers and phone outcome.
2. Occupy Ctrl+Alt+C with another application before launch: verify the fallback
   label and delivery. Occupy both: verify unavailable warning and working manual
   send, release the conflict and wait up to 30 seconds for recovery. Try text,
   image, large text, empty, sensitive/password-manager and temporarily locked
   clipboard. Confirm a real access failure is not labeled empty and does not
   block control-window polling. Close the phone: only queued status is expected.
3. Disconnect/reconnect Wi-Fi; deny browser clipboard permission; suspend/reopen
   the PWA. Correlate hotkey registration → received → clipboard read → outbox
   result with phone diagnostics. A published item can still require Copy on the
   phone. If a key produces no received log, verify registration/conflicts using
   the displayed label and an explicit key test; preserve logs around the failure.

Windows automated checks: `go test ./internal/pc ./internal/server ./cmd/copyapp`
and `go build ./cmd/copyapp`. New tests inject transient clipboard failures and
channel closure/burst behavior; they do not verify Win32 registration or a
physical keyboard. This environment has no Go toolchain, Windows session or
Android device, so compilation and tests must run in Windows CI; issue #3 must
remain open until the physical acceptance checks pass.
