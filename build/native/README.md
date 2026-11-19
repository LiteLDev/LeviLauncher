# macOS installer

Run from the repository root on macOS with the Xcode command-line tools,
the Go version in `go.mod`, Wails v3, Node.js/npm, and Python 3. Place the
Windows x64 loader at `internal/leviloader/LeviLauncher.dll`, or pass the
artifact to `macos.py` through `--loader`.

```sh
wails3 task package
```

This builds `bin/LeviLauncher.app` and creates
`bin/LeviLauncher-macos-installer.dmg`. To package the existing app in
`bin/`, run `wails3 task darwin:create:dmg`. To use a custom app or output path:

```sh
python3 build/native/dmg.py --app "/path/to/LeviLauncher.app" --output "/path/to/LeviLauncher.dmg"
```

Open the DMG and drag LeviLauncher into Applications. Set up Wine from
LeviLauncher as part of the Minecraft installation. The current app build
uses ad hoc code signing for local testing.
