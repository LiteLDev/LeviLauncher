# LeviLauncher

A desktop launcher for **Minecraft Bedrock Edition (GDK / UWP)** on **Windows**.

LeviLauncher helps you install, manage, isolate, mod, and launch Release or Preview builds while keeping content and tooling easier to organize.

This repository is the **Windows version** of LeviLauncher. The **Android version** is developed in [LiteLDev/LeviLaunchroid](https://github.com/LiteLDev/LeviLaunchroid); both are documented at https://docs.levimc.org/LeviLauncher/. There is no iOS version, and none is planned.

## Documentation

- **User docs:** https://docs.levimc.org/LeviLauncher/
- **中文文档：** https://docs.levimc.org/LeviLauncher/zh-CN/
- **Android version docs:** https://docs.levimc.org/LeviLauncher/android/

## Downloads

- **GitHub Releases:** https://github.com/LiteLDev/LeviLauncher/releases
- **Lanzou Cloud:** https://levimc.lanzoue.com/b016ke39hc (Password: `levi`)

## Requirements

- Windows 10/11
- A legitimate licensed copy of Minecraft Bedrock Edition
- Microsoft Edge WebView2 Runtime
- GDK: Microsoft Gaming Services and Microsoft GameInput available on the system
- UWP: Windows Developer Mode and the UWP framework dependencies required by the package manifest
- Native mod loader (`preload-native` DLL mods): the desktop Visual C++ runtime (x64), which the launcher checks and offers to install

UWP x64 instances support the console toggle under **Instance Settings → Launch Options** and compatible `preload-native` DLL mods imported through **Manage Mods**. Instance isolation is available for UWP x64 version 1.19.70.2 and later; non-isolated UWP instances keep using the app channel's shared `LocalState` content. Installation deploys the loader; launching a registered instance prepares native loading while preserving Windows app activation. Vanilla x86/ARM64 packages can still launch, but the bundled native loader currently supports x64 only. Automatic LeviLamina installation remains limited to supported GDK versions.

## Community

- **Discord:** https://discord.gg/v5R5P4vRZk
- **QQ Group:** https://qm.qq.com/q/1z791rJgJG

## Issue Reporting

- **Issues:** https://github.com/LiteLDev/LeviLauncher/issues
- Include your Windows version, LeviLauncher version, repro steps, and logs/screenshots when possible.

## Development

This repository's end-user guide is published at https://docs.levimc.org/LeviLauncher/. For local docs work:

```bash
cd docs
npm ci
npm run dev
```

For app development, keep using the existing `frontend/`, `build/`, and Wails task flow.

## License

This project is licensed under GPL-3.0-only. The full license text is in `COPYING`, and the bundled third-party components are listed in `THIRD_PARTY_NOTICES`.
