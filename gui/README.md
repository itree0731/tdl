# TDL Desktop GUI

Wails v2 desktop application. The existing root `tdl.exe` remains the CLI;
the GUI builds as a separate `tdl-gui.exe` and reuses the same Go transfer
packages.

```powershell
cd gui\frontend
npm install
npm run build
cd ..\..
go build -o tdl-gui.exe ./gui
```

The first checkpoint contains the D warm graphite/copper application shell.
Backend account, picker and transfer services are added behind typed bindings;
the frontend does not execute shell command strings.
