# Changelog

All notable changes to this project will be documented in this file.

## [v0.3.14] - 2026-08-05

### Added

- Added support for selecting a custom WebView2 Fixed Version Runtime through `--webview2-runtime-dir` or a `webview2.json` file next to the launcher executable ([#118](https://github.com/LiteLDev/LeviLauncher/issues/118)).
- Added automated accessibility coverage for core launcher workflows.

### Changed

- Streamlined frontend startup, routing, animations, and state initialization while improving keyboard and screen-reader accessibility.
- Batched backend metadata requests and streamed local image assets to reduce startup and content-management overhead.

### Fixed

- Validated imported mod manifests and entry files before extraction, and made archive replacement transactional so invalid packages cannot overwrite existing mods ([#114](https://github.com/LiteLDev/LeviLauncher/issues/114)).
- Serialized downloader task lifecycle transitions to prevent duplicate terminal events and inconsistent task state.
- Restored the bundled MiSans font and its required typography fallbacks.
- Restored readable white foregrounds for primary buttons across custom themes.

### CI

- Enforced backend tests, frontend production bundle limits, and accessibility checks in the build workflow.
- Restored complete cross-platform optional dependency metadata in the frontend lockfile so clean `npm ci` installs remain reproducible.

### Docs

- Documented custom WebView2 Fixed Version Runtime configuration for English and Simplified Chinese users.

### Chore

- Bumped version to `0.3.14`.

## [v0.3.13] - 2026-06-26

### Fixed

- Fixed a crash on Minecraft Bedrock `1.26.31` by updating the bundled `vcruntime140_1.dll`.

### CI

- Fixed Windows build workflow configuration.

### Chore

- Bumped version to `0.3.13`.

## [v0.3.12] - 2026-06-06

### Added

- Added a debug mode startup path for collecting richer launcher diagnostics.
- Added a WebView2 runtime startup check before the launcher UI initializes.
- Added editing support for normal mod metadata on the Mods page.

### Changed

- Updated Japanese localization coverage.

### Fixed

- Handled Visual C++ runtime startup installation failures more reliably.
- Localized startup failure diagnostics shown before the launcher UI appears.
- Allowed closing toast notifications from the launcher UI.

## [v0.3.11] - 2026-05-04

### Added

- Added startup diagnostics that write `%APPDATA%\levilauncher.exe\logs\startup.log` and surface the log path when the launcher fails before the UI appears.

### Changed

- Started WebView2 with safer software-rendered GPU settings to reduce startup crashes on affected Windows GPU and driver combinations.

### Fixed

- Normalized detected locale aliases, including script and region variants, before selecting launcher translations.
- Corrected English download page typos in version search and version totals.

### Docs

- Documented startup crash recovery steps and startup log reporting in the troubleshooting guides and bug report template.

### Chore

- Bumped version to `0.3.11`.

## [v0.3.10] - 2026-04-07

### Added

- Added German, Spanish, French, Italian, Korean, and Portuguese locale support across the launcher.(AI)

### Changed

- Optimized the lip download experience for users in China and streamlined related launcher settings and workflows.
- Refined the update modal changelog layout for better readability.
- Cleaned up locale keys and refreshed the repository sync scripts.

### Fixed

- Prevented invalid dropped file paths from breaking frontend file handling.
- Improved Visual C++ runtime detection stability.
- Shortened lip install timeouts to reduce stalled installation flows.
- Surfaced lip cache clean results directly in Settings.
- Removed the forced GDK `1.26.0.0` registration override to avoid incorrect registrations.
- Enforced exact LeviLamina-to-game-version mapping during install and backup compatibility checks.
- Waited for the frontend dev server before launching the desktop app in local development tasks.
- Restored monospace font fallbacks to avoid incorrect font rendering in the frontend.

### Docs

- Clarified the Lanzou password in quick start guides.
- Fixed broken links in the Russian documentation.
- Expanded the documentation site with German, Spanish, French, Japanese, and Korean translations.

### Chore

- Bumped version to `0.3.10`.
- Bumped frontend dependency `picomatch` to `4.0.4`.

## [v0.3.8] - 2026-03-18

### Added

- Added an experimental instance backup and restore workflow in instance settings, including backup scope selection and restore conflict handling.

### Changed

- Streamlined the launcher startup flow and kept startup prompts aligned with onboarding progress.
- Gated instance backup behind an explicit experimental feature toggle in Settings.
- Optimized the frontend startup bundle, introduced desktop-safe runtime shims, and unified typography across the app.
- Refined lip package, instance state, and modal handling across the launcher UI.

### Fixed

- Prevented the Mods page from being blocked while lip status is still loading, with a safer local-only fallback when lip data is unavailable.
- Prevented launcher startup from stalling when system proxy discovery fails offline.
- Refreshed the user session during launcher startup.
- Added desktop runtime fallback assets to improve packaged frontend startup resilience.
- Guarded Wails resize flag initialization and avoided nested buttons in instance cards.
- Hardened modal behavior for critical flows and hid modal close buttons by default.

### Docs

- Expanded the documentation site with Russian and Traditional Chinese (Hong Kong) translations and updated the multilingual docs configuration.

## [v0.3.7] - 2026-03-11

### Added

- Added LeviLamina version selection, update, and uninstall confirmation flows.
- Added privacy settings with a Microsoft Clarity consent flow.
- Added screenshot preview with in-app navigation.
- Added more launcher tips.

### Changed

- Revamped lip-aware mod and package workflows, including instance package management and version registration.
- Improved launcher-side lip package management and version registration.
- Embedded `lipd` to support local daemon-based lip operations.
- Reorganized instance-related pages and routes.

### Fixed

- Reported early startup failures before the game window appears.
- Allowed downloading other installers while one installer task is active.
- Preserved drag-restore behavior while suppressing resize cursors.
- Hardened update and launch safety, and stabilized the update and settings experience.

## [v0.3.6] - 2026-02-25

### Fixed

- Disabled launcher zoom gestures on touch devices to prevent accidental UI scaling.

## [v0.3.5] - 2026-02-25

### Added

- Added cross-version transfer APIs for packs and worlds in the backend.
- Added resource/world transfer flows for isolated instances.
- Added LeviLamina detection and registered-state synchronization.

### Changed

- Revamped the LIP package index and details page.
- Polished primary accents and custom theme color preview.
- Centralized table header styles in the frontend.
- Tweaked compiler flags to reduce antivirus false positives.
- Removed dead code.

### Fixed

- Prevented launch when cancel is clicked in the force-start dialog.
- Enforced minimum window size after restore and synchronized the maximize icon state.
- Kept background image size stable when toggling blur.
- Resolved desktop path from `User Shell Folders` for shortcut handling.
- Showed unregister progress modal on the home page.
- Removed hover haze on option rows.
- Unified content management page control styles.

### Chore

- Bumped version to `0.3.5`.
