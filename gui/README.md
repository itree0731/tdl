# TMT Desktop GUI

Wails v2 desktop application. The root `tmt.exe` is the CLI;
the GUI builds as a separate `tmt-gui.exe` and reuses the same Go transfer
packages.

```powershell
.\scripts\build-gui.ps1
```

The script builds the React frontend and then compiles Wails with its required
`production` build tag. A plain `go build ./gui` creates an executable that can
only display Wails' missing-build-tags error, so do not use it for delivery.

The current desktop build provides:

- native file and directory pickers;
- real upload and download commands with structured progress and cancellation;
- paged Telegram chat/topic selection;
- `video_cover` upload options with playback starting at zero;
- validated, atomically saved GUI settings in `~/.tdl/tmt.json`.

The frontend calls typed Wails bindings and never assembles shell command
strings. `tmt.exe` is the standalone CLI; `tmt-gui.exe` is the desktop app.
Existing accounts remain in the legacy-compatible `~/.tdl` data directory.
