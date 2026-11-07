# LeviLauncher

A desktop launcher for **Minecraft Bedrock Edition (GDK / UWP)** on **Windows**.

## Background

LeviLauncher helps you install, manage, isolate, mod, and launch Release or Preview builds while keeping content and tooling easier to organize.

This repository is the **Windows version** of LeviLauncher. The **Android version** is developed in [LiteLDev/LeviLaunchroid](https://github.com/LiteLDev/LeviLaunchroid); both are documented in one guide at https://docs.levimc.org/LeviLauncher/ (Windows / Android tabs). Developer docs for Android native mods are in the [LeviLamina docs](https://docs.levimc.org/LeviLamina/developer_guides/android/). There is no iOS version, and none is planned.

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

`LeviLauncher.exe` with no arguments opens the launcher window; with a subcommand it runs headless and exits:

```text
LeviLauncher.exe list                   List installed instances
LeviLauncher.exe launch <instance>      Launch an instance
LeviLauncher.exe call                   List every backend method the GUI uses
LeviLauncher.exe call <method> [args]   Invoke one of them
LeviLauncher.exe help                   Show usage
```

Everything the GUI does goes through one of the methods `call` lists, so the command line can do all of it (download, install, import content, manage mods, settings and so on). String parameters take the argument as is, other types take JSON (`true`, `42`, `["a"]`). Results go to stdout (JSON unless a plain string), progress events to stderr as JSON lines, and downloads are waited for. The exit status is 0 on success, 1 when the call fails (a Go error or an `ERR_...` code) and 2 on a usage error.

`LeviLauncher.exe` is a GUI program, so shells don't wait for it by default. When you need its output before the prompt returns, or its exit code, use `start /wait LeviLauncher.exe ...` in cmd. In PowerShell, pipe the output (`LeviLauncher.exe list | Out-String`) or use `Start-Process -Wait -NoNewWindow`.

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
