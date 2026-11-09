# LeviLauncher

A desktop launcher for **Minecraft Bedrock Edition (GDK / UWP)** on **Windows**.

## Background

LeviLauncher helps you install, manage, isolate, mod, and launch Release or Preview builds while keeping content and tooling easier to organize.

This repository is the **Windows version** of LeviLauncher. The **Android version** is developed in [LiteLDev/LeviLauncher-Android](https://github.com/LiteLDev/LeviLauncher-Android); both are documented in one guide at https://docs.levimc.org/LeviLauncher/ (Windows / Android tabs). Developer docs for Android native mods are in the [LeviLamina docs](https://docs.levimc.org/LeviLamina/developer_guides/android/). There is no iOS version, and none is planned.

## Install

### Requirements

- Windows 10/11
- A legitimate licensed copy of Minecraft Bedrock Edition
- Microsoft Edge WebView2 Runtime
- GDK: Microsoft Gaming Services and Microsoft GameInput available on the system
- UWP: Windows Developer Mode and the UWP framework dependencies required by the package manifest
- Native mod loader (`preload-native` DLL mods): the desktop Visual C++ runtime (x64), which the launcher checks and offers to install

UWP x64 instances support the console toggle under **Instance Settings → Launch Options** and compatible `preload-native` mods imported through **Manage Mods** as `.mcmod` packages (`.zip` packages and bare `.dll` files can still be picked via **All files**). Instance isolation is available for UWP x64 version 1.19.70.2 and later; non-isolated UWP instances keep using the app channel's shared `LocalState` content. Installation deploys the loader; launching a registered instance prepares native loading while preserving Windows app activation. Vanilla x86/ARM64 packages can still launch, but the bundled native loader currently supports x64 only. Automatic LeviLamina installation remains limited to supported GDK versions.

### Downloads

- **GitHub Releases:** https://github.com/LiteLDev/LeviLauncher/releases
- **Lanzou Cloud:** https://levimc.lanzoue.com/b016ke39hc (Password: `levi`)

## Usage

See the user docs:

- **User docs:** https://docs.levimc.org/LeviLauncher/
- **中文文档：** https://docs.levimc.org/LeviLauncher/zh-CN/

### Command Line

`LeviLauncher.exe` with no arguments opens the launcher window. With a command it runs in the terminal and exits, using a `<noun> <verb>` grammar:

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
  backup      list, create, restore

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

- **Issues:** https://github.com/LiteLDev/LeviLauncher/issues
- Include your Windows version, LeviLauncher version, repro steps, and logs/screenshots when possible.

### Community

- **Discord:** https://discord.gg/v5R5P4vRZk
- **QQ Group:** https://qm.qq.com/q/1z791rJgJG

### Development

This repository's end-user guide is published at https://docs.levimc.org/LeviLauncher/. For local docs work:

```bash
cd docs
npm ci
npm run dev
```

For app development, keep using the existing `frontend/`, `build/`, and Wails task flow.

## License

This project is licensed under GPL-3.0-only. The full license text is in `COPYING`, and the bundled third-party components are listed in `THIRD_PARTY_NOTICES`.
