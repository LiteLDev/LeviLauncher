# Contributing to LeviLauncher

Thank you for helping. This file is for people who work on LeviLauncher itself: the app and its documentation.
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

The end-user guide in `docs/` is published at https://levimc.org/LeviLauncher/. It is built with levimc-kit, which
`npm ci` downloads from levimc.org into `docs/kit/`. For local docs work:

```bash
cd docs
npm ci
npm run dev
```

Before a pull request, run what the `docs` workflow runs:

```bash
npm run typecheck
npm run build
npm run linkcheck
```

To try changes of levimc-kit that are not published yet, point the install at a kit checkout:
`LEVIMC_KIT_SOURCE=../path/to/liteldev.github.io/kit npm run kit`.

Where the pages are:

- English is in `docs/` and is published at the root. The other languages are in folders named after their locale
  (`docs/zh-CN/`, `docs/zh-HK/`, `docs/es-ES/`, …). Simplified Chinese (`zh-CN/`) is published at `zh-Hans/`,
  Traditional Chinese (`zh-HK/`) at `zh-Hant/`; the others at their folder's name.
- The site has guides and reference only. Tutorials for newcomers, such as installing LeviLauncher and playing with
  mods for the first time, are on https://levimc.org/tutorials/ and are written in the LiteLDev/liteldev.github.io
  repository.
- A page that moves or goes away keeps its old URLs working: add them to `redirects` in `docs/docs.config.ts`.
