# Changelog

Notable changes to Agent Session Manager, newest first.

Entries describe what changed for someone using the app. Internal refactoring,
test and CI work is left out unless it changed behaviour. Dates are release
dates; the format follows [Keep a Changelog](https://keepachangelog.com).

This file was started at 0.8.0. Earlier releases are summarised from their
commits and are shorter for that reason, not smaller. Point releases that only
carried a fix or two are folded into the version above them.

## 0.8.0 — 2026-08-08

### Added

- **Codex activity is detected.** Codex reports progress as a static
  "Working" line rather than an animated spinner, and spinner animation was
  the only busy signal a non-Claude agent had — so a Codex agent hard at work
  read as idle throughout. It now has a detector that looks for what it
  actually prints.
- **Detection patterns live in a file that updates itself.** The phrases that
  mean "waiting for you" used to be compiled into the binary, so when Claude
  or Codex reworded a prompt the app stopped noticing that agent waiting for
  an answer, and only a new release could fix it. The patterns now ship
  embedded as a fallback, refresh from the repository once a day, and
  `--refresh-patterns` fetches them immediately. A newer file only ever
  replaces an older one, so a stale download cannot undo a fix shipped in the
  binary.

### Fixed

- **Actions could hit the wrong window, losing work.** Several places assumed
  the agent was window 0 of its tmux session. tmux does not renumber windows,
  so that is not true in general — and tmux does not fail on a missing numeric
  target either: it falls back to the *current* window and reports success.
  Closing a tab, switching an agent to a shell, or resuming a session could
  therefore act on whatever tab you happened to be looking at. The agent's
  window is now identified explicitly.
- **The interface stopped answering the keyboard with several busy sessions.**
  Every window costs a tmux capture, and a window showing a spinner costs a
  second one after a short wait — the only way to tell a live spinner from one
  frozen in scrollback is to look twice. All of it ran inline in the update
  loop, so five sessions of three busy tabs spent nearly two seconds per tick
  against a 100ms tick, and the queue never caught up. Sessions are now probed
  off the UI thread and concurrently with each other.
- **A failed capture read as an idle agent.** Detection now reports whether it
  could see the pane at all, so the two are distinguishable.

## 0.7.8 — 2026-01-24

### Fixed

- **Waiting detection** matched dialog content left further up the pane, so a
  session could show as waiting after you had already answered. It now looks
  below the separator, and the session list says "Waiting" rather than leaving
  you to infer it.

## 0.7.7 — 2026-01-24

### Fixed

- **tmux key bindings applied to every session**, not just the app's own, and
  the conditional form used the wrong session-name prefix so it did not match.
- **Stopping a tab** now interrupts the agent rather than killing the window,
  so the session can be resumed afterwards.
- **Resume → Replace Current** replaced the whole session instead of the
  current tab; **Resume → New Tab** now asks what to call it.
- Status line detection: spinner content, completed lines and `⎿`
  continuations are no longer shown as the agent's status.
- A stopped tab could not be resumed with Enter.
- Input field widths in the name and path dialogs.

## 0.7.5 — 2026-01-11

### Added

- **Global history search** across several agents' histories at once.
- **Fork a session** — branch a Claude conversation into a new tab or a new
  session.
- **Favorites** — mark the sessions you keep returning to; they group together
  at the top.
- **Update notifications** on the project screen, with the check cached so it
  does not run on every visit.

## 0.6.5 — 2026-01-07

### Added

- **Diff view** — review a session's Git changes without leaving the app.
- **Per-tab notes.**

## 0.6.1 — 2026-01-07

### Added

- **Multi-tab sessions** — several agents or terminals per session, each in its
  own tmux window.
- **A tmux status bar** carrying the tabs and the YOLO indicator.
- **Session notes.**
- **Agent icons**, and improvements to the dialogs and project backgrounds.

### Fixed

- The `.deb` download URL used the wrong architecture name.

## 0.4.3 — 2026-01-05

### Added

- **YOLO mode toggle** on `Ctrl+Y`, prompt suggestions, and activity status.
- **Waiting status detection**, and a better Claude status line.

### Fixed

- Project screen handling and error reporting.

## 0.3.7 — 2026-01-05

### Added

- **Self-update** with `.deb`, `.rpm` and tarball support.
- **Multi-agent resume**, and a key to start a session immediately.
- **Parallel session start**, and better handling of wide characters.

## 0.3.0 — 2026-01-03

### Added

- **Projects and workspaces**, with a lock so a second copy of the app cannot
  fight the first one over the same sessions.
- A scrollable preview, and a steadier layout.

## 0.2.0 — 2026-01-01

### Added

- **Self-update**, and an install script.
- **Split view** for comparing two sessions.
- A session status counter in the header.

## 0.1.0 — 2025-12-31

First tagged release.
