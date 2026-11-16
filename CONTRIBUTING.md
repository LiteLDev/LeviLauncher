# Contributing to LeviLauncher

Thank you for helping. This file is for people who work on LeviLauncher itself.
If you only want to use LeviLauncher, read the user guide at https://levimc.org/LeviLauncher/.

## Reporting issues

- **Issues:** https://github.com/LiteLDev/LeviLauncher/issues
- Include your Windows version, LeviLauncher version, repro steps, and logs/screenshots when possible.

Questions are also welcome on [Discord](https://discord.gg/v5R5P4vRZk) and in the [QQ group](https://qm.qq.com/q/1z791rJgJG).

## Development

### The app

For app development, keep using the existing `frontend/`, `build/`, and Wails task flow (`Taskfile.yml`), as the
`build` workflow in `.github/workflows/` does.

### The documentation

The end-user guide is published at https://levimc.org/LeviLauncher/; the other languages are at
`https://levimc.org/<language>/LeviLauncher/`, for example https://levimc.org/zh-Hans/LeviLauncher/. Its pages are
not in this repository: they are in
[`docs/LeviLauncher/` of LiteLDev/liteldev.github.io](https://github.com/LiteLDev/liteldev.github.io/tree/main/docs/LeviLauncher),
which builds and publishes the whole levimc.org site. Open documentation issues and pull requests there.

The site has guides and reference only. Tutorials for newcomers, such as installing LeviLauncher and playing with
mods for the first time, are on https://levimc.org/tutorials/ and are written in the same repository.
