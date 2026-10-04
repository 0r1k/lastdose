# LastDose

A terminal (TUI) counter of time spent free from a bad habit: smoking or alcohol.
Written in Go with Bubble Tea.

```sh
go build -o lastdose .
./lastdose
```

## Time you can't cheat

LastDose never reads the computer clock to count progress.

- On launch it syncs with NTP servers (`time.cloudflare.com`, `time.google.com`,
  `pool.ntp.org`) and takes the median answer. HTTPS `Date` headers are the
  fallback for networks that block UDP 123.
- After the sync, time advances on Go's monotonic clock, which ignores changes
  to the system date. A detected jump (manual change, suspend) triggers a resync,
  and the app also resyncs every 10 minutes.
- An internet connection is required to start. Without one, the app waits on
  the sync screen and retries every 10 seconds. If the network drops later, it
  keeps counting on the monotonic clock.
- If the system clock is off by more than a minute, the app shows that the
  offset is ignored.

`state.json` stores the start moment, taken from network time. Elapsed time is
always `network now - start`, so nothing runs in the background. `last_seen` is
saved on exit, every 10 seconds, and on SIGHUP/SIGTERM/SIGQUIT (for example, when
the terminal window is closed).

## Time zone

The zone is detected online from your public IP (`ipinfo.io`, `ipapi.co`) and
offered as the default. The system zone, a list of common zones and manual
entry are also available. The zone only changes how dates are displayed.

## Relapse

Pressing `r` and confirming wipes the habit completely. The counter and every
badge are deleted, and no best streak or history is kept. To track it again,
start the counter again from scratch.

## Badges

There are 12 steps: 24 hours, then each step doubles. The last badge unlocks
after 2048 days (~5.6 years), which covers 5 years with room to spare. Names and
mottos are in `internal/achievements/achievements.go`. The ASCII owl and the
per-badge hats are in `internal/art/owl.go`.

## Final congratulation

The modal after the last badge shows, in order of priority:

1. `final_message.txt` next to `state.json`, if present (no rebuild needed);
2. `assets/final_message.txt`, embedded into the binary at build time.

## Keys

| Screen    | Keys                                                                   |
|-----------|------------------------------------------------------------------------|
| Dashboard | `tab` habit, `a` badges, `n` add habit, `t` time zone, `r` relapsed, `q` quit |
| Badges    | `j/k` browse, `esc` back                                               |
| Popup     | `enter` accept, `s` accept all                                         |

## Flags

```sh
./lastdose -data path/to/state.json   # another state file
./lastdose -demo 100h                 # throwaway state, smoking quit 100h ago
./lastdose -preview-final             # show the final modal right away
```

`-demo` works on a temporary file that is deleted on exit, so your real progress
is never touched.

## License

Copyright (C) 2026 orik

LastDose is free software: you can redistribute it and/or modify it under the
terms of the [GNU General Public License v3.0](LICENSE) as published by the Free
Software Foundation. Anyone who distributes LastDose or a modified version must
release the source code under the same license.
