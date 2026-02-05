![LeviLauncher](https://socialify.git.ci/LiteLDev/LeviLauncher/image?custom_language=Go&description=1&font=Inter&forks=1&issues=1&language=1&logo=https%3A%2F%2Fgithub.com%2FLiteLDev%2FLeviLauncher%2Fblob%2Fmain%2Fbuild%2Fappicon.png%3Fraw%3Dtrue&name=1&owner=1&pattern=Plus&pulls=1&stargazers=1&theme=Auto)

<p align="center">
  <a href="https://discord.gg/v5R5P4vRZk"><img alt="Discord" src="https://img.shields.io/discord/849252980430864384?style=for-the-badge&logo=discord"></a>
  <a href="https://qm.qq.com/q/1z791rJgJG"><img alt="QQ Group 458083875" src="https://img.shields.io/badge/458083875-red?style=for-the-badge&logo=qq"></a>
</p>

<p align="center">
  <sup>🌐 Language: <b>English</b> • <a href="./README.zh-CN.md">中文</a></sup>
</p>

A desktop launcher for Minecraft Bedrock Edition (GDK) on Windows.

Supports installing, managing, and launching Release/Preview builds. Provides version isolation, content management (worlds/resource packs/behavior packs/skin packs), mod management, and world backup tools. Frontend uses React + Vite + Tailwind; backend uses Go with Wails v3.

## Project Status
- 🚧 Actively under development; features may be incomplete and unstable.

## Scope
- Targets Minecraft GDK (Windows). Requires a legitimate licensed game copy.

## Downloads
- GitHub Releases: https://github.com/LiteLDev/LeviLauncher/releases
- Lanzou Cloud: https://levimc.lanzoue.com/b016ke39hc (Password: `levi`)

## Issue Reporting
- Open issues at https://github.com/LiteLDev/LeviLauncher/issues
- Include OS version, launcher version, repro steps, and logs/screenshots.

## Features
- Version management: install, delete, rename, quick launch; supports Release and Preview.
- Version isolation: redirect game data to `versions/<name>/Minecraft Bedrock (Preview)`, separate from AppData.
- Content management: count and manage worlds/resource/behavior/skin packs; quick open and drag-and-drop import.
- Mods: import `.zip`/`.dll`, enable/disable/delete; auto prepare dependencies and preloader.
- World tools: backup to `.mcworld`, edit `level.dat` fields and world name.
- Downloads & mirrors: built-in mirror latency test and selection; local installer import; progress view.
- Shortcuts: create desktop shortcut for a version; custom icon support.
- Updates: check, download, install updates; elevate when necessary.
- Languages: English and Simplified Chinese.

## Requirements
- OS: Windows 10/11.
- Required components: Microsoft Gaming Services, Microsoft GameInput (guided on first run).
- WebView2 Runtime: provided by installer or system.
- Minecraft: You must own a legitimate licensed copy and have installed it at least once from Microsoft Store to use the launcher for installation.

## Quick Start (Dev)
- Dependencies:
  - Go `1.24+` (see `go.mod`).
  - Node.js `18+` (for frontend).
  - Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- Dev mode:
  - From project root: `wails3 dev -config ./build/config.yml -port 1145`
  - Or run frontend separately: `cd frontend && npm install && npm run dev`
- Build:
  - `wails3 task build`
  - Windows specific: `wails3 task windows:build`
- Run:
  - `wails3 task run`

## Structure
- `frontend/`: React + Vite app (`package.json`, `src/`, `assets/locales/`).
- `internal/`: backend logic (versions, content, update, registry, etc.).
- `build/`: cross-platform packaging and Taskfiles.
- `main.go`: entry point; embeds frontend assets and creates the window.

## CLI Args
- `--launch=<version_name>`: launch the specified version and exit.
- `--self-update=<current_version>`: start elevated self-update when the install dir is not writable.

## Community
- Discord: `https://discord.gg/v5R5P4vRZk`
- QQ Group: `458083875` (`https://qm.qq.com/q/1z791rJgJG`)

## FAQ
- Missing GameInput: install `GameInputRedist.msi` when prompted.
- Missing Gaming Services: install via Microsoft Store.
- Isolation & inherit: enable isolation during install; copy data from same-type isolated version or GDK directory.
- Non-writable directory: change base content path in Settings or install/update with elevation.
- Language switch: Settings supports English/zh-CN.

## Contributing

- PRs and issues are welcome.

## License

Copyright © 2025 LeviMC, All rights reserved.

This project is licensed under the LGPL-3.0 License for its non-closed source parts - see the [COPYING](COPYING) and [COPYING.LESSER](COPYING.LESSER) files for details.

