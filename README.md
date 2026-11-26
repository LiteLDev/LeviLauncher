# LeviLauncher

A desktop launcher for **Minecraft Bedrock Edition** on **Windows and macOS**. Windows supports GDK and UWP. The macOS launcher runs the GDK game through Wine.

## Background

LeviLauncher helps you install, manage, isolate, mod, and launch Release or Preview builds while keeping content and tooling easier to organize.

The **Android version** is developed in [LiteLDev/LeviLauncher-Android](https://github.com/LiteLDev/LeviLauncher-Android). The [LeviLauncher guide](https://levimc.org/LeviLauncher/) covers Windows and Android. Developer docs for Android native mods are in the [LeviLamina docs](https://levimc.org/LeviLamina/developer_guides/android/). There is no iOS version, and none is planned.

## Install

### Downloads

- **GitHub Releases (Windows & macOS):** <https://github.com/LiteLDev/LeviLauncher/releases>
- **Lanzou Cloud (Windows):** <https://levimc.lanzoue.com/b016ke39hc> (Password: `levi`)

### Windows

Requirements:

- Windows 10/11
- A legitimate licensed copy of Minecraft Bedrock Edition
- Microsoft Edge WebView2 Runtime
- GDK: Microsoft Gaming Services and Microsoft GameInput available on the system
- UWP: Windows Developer Mode and the UWP framework dependencies required by the package manifest
- Native mod loader (`preload-native` DLL mods): the desktop Visual C++ runtime (x64), which the launcher checks and offers to install

UWP x64 instances support the console toggle under **Instance Settings → Launch Options** and compatible `preload-native` mods imported through **Manage Mods** as `.mcmod` packages (`.zip` packages and bare `.dll` files can still be picked via **All files**). Instance isolation is available for UWP x64 version 1.19.70.2 and later; non-isolated UWP instances keep using the app channel's shared `LocalState` content. Installation deploys the loader; launching a registered instance prepares native loading while preserving Windows app activation. Vanilla x86/ARM64 packages can still launch, but the bundled native loader currently supports x64 only. Automatic LeviLamina installation remains limited to supported GDK versions.

**Build from source (Windows):** requires the Go version in `go.mod`, Wails v3, Node.js/npm, xmake (`3.0+`) with MSVC (for the native loader), and Inno Setup (`iscc`, only needed to build the installer). First build and deploy the native loader to `internal/leviloader/LeviLauncher.dll` from `native/levilauncher`:

```sh
xmake f -p windows -a x64 -m release --toolchain=msvc --deploy=y -y
xmake -y
```

Then from the repository root, run `wails3 task build PRODUCTION=true` to build `bin/LeviLauncher.exe`, or `wails3 task package` to also create `bin/LeviLauncher-amd64-installer.exe`.

### macOS

Requirements:

- macOS 14.6 or later (prebuilt releases target Apple Silicon `arm64`)
- A legitimate licensed copy of Minecraft Bedrock Edition

Download `LeviLauncher-macos-arm64-installer.dmg` from GitHub Releases, open it, and drag **LeviLauncher** into **Applications**.

macOS runs GDK builds through Wine (including LeviLamina, LIP packages, and compatible native mods) and supports the terminal CLI (`levilauncher` when installed via Homebrew, or `/Applications/LeviLauncher.app/Contents/MacOS/LeviLauncher`); UWP instances are currently Windows-only.

**Build from source (macOS):** requires Xcode Command Line Tools, the Go version in `go.mod`, Wails v3, Node.js/npm, Python 3, and the Windows x64 loader at `internal/leviloader/LeviLauncher.dll` (or passed to `build/native/macos.py` via `--loader`). Run `wails3 task package` from the repository root to build `bin/LeviLauncher.app` and `bin/LeviLauncher-macos-installer.dmg`. To package an existing `.app` bundle, run `wails3 task darwin:create:dmg` or `python3 build/native/dmg.py --app "/path/to/LeviLauncher.app" --output "/path/to/LeviLauncher.dmg"`.

## Usage

See the user docs:

- **User docs:** <https://levimc.org/LeviLauncher/>
- **中文文档：** <https://levimc.org/zh-Hans/LeviLauncher/>

### Command Line

Running `LeviLauncher.exe` (Windows) or `levilauncher` (macOS) with no arguments opens the launcher window. With a command it runs in the terminal and exits, using a `<noun> <verb>` grammar:

```text
Everyday:
  play [instance]              Start the game and wait until its window is up (alias: launch)
  install <version|file>       Download and install Minecraft as a new instance (= instance create)
  use [instance]               Set or show the default instance
  stop [instance|pid]          Close a running Minecraft (= process stop)

Manage an instance:
  instance    list, show, create, copy-data, rename, set, remove, register, unregister, shortcut, path
  mod         list, show, search, install, update, import, set, remove, enable, disable, path
  loader      versions, install, remove (LeviLamina)
  world       list, import, export, copy, rename, set, get, remove
  pack        list, import, copy, remove
  screenshot  list, remove
  server      list
  player      list
  backup      list, create, restore (preview)

Find things to install:
  installer   list, search, download, remove, mirrors (Minecraft builds)
  curseforge  show, search, categories, install (alias: cf)

This PC and the launcher:
  process     list, stop (alias: ps)
  account     status, login, logout
  config      list, set, get, unset
  cache       clean
  doctor      Check what Minecraft and mods need on this PC; --fix repairs it
  self-update Update LeviLauncher itself
  version     Print version information
  help        Help for any command, with examples
```

For example: `LeviLauncher.exe install latest --loader --use`, `LeviLauncher.exe play`, `LeviLauncher.exe mod install LiteLDev/LegacyMoney`, `LeviLauncher.exe world export "My World" -o D:\Backups`. Run `LeviLauncher.exe help <command>` for the flags and examples of each command.

Global flags: `-i/--instance <name>`, `-p/--player <id|gamertag>`, `--json` (one JSON document on stdout), `-q/--quiet` (identifiers only), `-v/--verbose` (also the launcher log on stderr), `-y/--yes`, `--no-input`, `-h/--help`, `--version`. Results go to stdout; progress, notices and errors (`error:`, `hint:`, `[ERR_CODE]`) go to stderr. Exit codes: 0 ok, 1 failed, 2 usage, 3 not found, 4 conflict, 5 sign-in/licence, 6 network, 7 partial, 8 needs a system change or admin, 9 timeout, 10 cancelled, 130 interrupted. The environment variables `LEVILAUNCHER_INSTANCE` and `LEVILAUNCHER_PLAYER` set a default instance and player for a shell. The window's own switches (`--launch=<instance>`, `--debug`, ...) keep opening the window.

`LeviLauncher.exe` is a GUI program, so shells don't wait for it by default. When you need its output before the prompt returns, or its exit code, use `start /wait LeviLauncher.exe ...` in cmd. In PowerShell, pipe the output (`LeviLauncher.exe instance list | Out-String`) or use `Start-Process -Wait -NoNewWindow`.

## Contributing

### Issue Reporting

- **Issues:** <https://github.com/LiteLDev/LeviLauncher/issues>
- Include your OS version (Windows or macOS), LeviLauncher version, repro steps, and logs/screenshots when possible.

### Community

- **Discord:** <https://discord.gg/v5R5P4vRZk>
- **QQ Group:** <https://qm.qq.com/q/1z791rJgJG>

### Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for working on the app and its documentation.

## License

This project is licensed under GPL-3.0-only. The full license text is in `COPYING`, and the bundled third-party components are listed in `THIRD_PARTY_NOTICES`.
