# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Changed

- Switched the LeviMC Mods package index from `lipr.levimc.org/levilauncher.json` to `levimc.org/mods/index.json` and per-mod `levimc.org/mods/<tooth>/index.json`

## [1.0.5] - 2026-10-28

### Added

- Added GitHub proxy on settings page

### Changed

- Removed lipConfigGoModuleProxy because it has been removed from lip v0.35.0
- Bumped wails3 to v3.0.0

### Fixed

- Made `isChinaUser()` more reliable

## [1.0.4] - 2026-10-27

### Changed

- Brought the user docs in every language in line with the app: GDK / UWP support (UWP instance isolation from 1.19.70.2 on x64), the app's own labels for pages and settings tabs, lowercase lip as the package installer, the Visual C++ runtime requirement, and page titles matching the sidebar; added the UWP, WebView2 Fixed Version Runtime, wallpaper material and startup-crash sections to all translations.
- Renamed the Bedrinth mod catalog to LeviMC Mods (Chinese: LeviMC 模组 / LeviMC 模組) in every language, matching LeviMC Mods (levimc.org/mods/); its search box now reads "Search mods…".
- Switched lip npm package from @futrime/lip to @levimc/lip

### Removed

- Removed the unused client for the retired `api.bedrinth.com` service; the mod catalog already loads from `lipr.levimc.org`.

## [1.0.3] - 2026-09-22

### Added

- Added Polish localization ([#131](https://github.com/LiteLDev/LeviLauncher/issues/131)).
- Added an administrator indicator in the launcher interface and window title, with an explanation of administrator-mode sign-in behavior.

### Changed

- Bumped version to `1.0.3`.
- Ignored the Visual Studio `.vs` workspace folder.

### Fixed

- Fixed Microsoft/Xbox sign-in when running as administrator by opening the window-scoped Windows authentication prompt directly, while retaining the account picker for normal launches.
- Retried sign-in with an explicit authentication prompt when Windows requires user interaction without displaying the sign-in dialog, and improved sign-in error reporting.
- Restored the launcher logo on the startup loading and failure screens ([#133](https://github.com/LiteLDev/LeviLauncher/issues/133)).

## [1.0.2] - 2026-09-16

### Added

- Added discovery of the latest GDK Release and Preview versions directly from Xbox services when signed in, making new versions available before the historical catalog is updated.

### Changed

- Kept newly discovered GDK versions available across refreshes and launcher restarts until they are included in the historical catalog.
- Saved completed Xbox authentication key checks per UWP instance to avoid rescanning the game executable on subsequent launches.
- Lightened dialog backdrops in the light theme to keep translucent dialogs from appearing overly gray.
- Bumped version to `1.0.2`.

### Fixed

- Fixed Xbox sign-in in older UWP versions by updating outdated authentication keys during installation and before launching existing instances.
- Preserved game executables that already contain the current authentication key, including versions that contain both the old and new keys.
- Fixed unresponsive player selection and overwrite confirmation dialogs during content imports, and prevented repeated file drops from interrupting an active import.
- Made dialog borders, inset panel borders, and icon backgrounds follow the configured material opacity when a wallpaper is active.
- Fixed self-updates exiting prematurely during administrator elevation or the post-update restart by waiting for the previous launcher instance to close, and reporting a startup error if the wait times out.

## [1.0.1] - 2026-09-14

### Changed

- Bumped version to `1.0.1`.

### Fixed

- Omitted package signatures when extracting UWP installations so they can be registered as development packages.

## [1.0.0] - 2026-09-13

### Added

- Added UWP support for Minecraft Bedrock Edition.
- Added Microsoft account sign-in, showing your Xbox profile and your Release and Preview entitlements, and used for version downloads.
- Added configurable launcher behavior on game launch and exit, together with a system tray icon.
- Added interface material personalization, with presets, opacity, blur, wallpaper overlay, and readability protection saved separately for the light and dark themes.
- Added search in Settings.
- Added an installation prompt when an installer download finishes.
- Added support for a custom WebView2 Fixed Version Runtime through `--webview2-runtime-dir` or a `webview2.json` file next to the launcher executable ([#118](https://github.com/LiteLDev/LeviLauncher/issues/118)).
- Added recognition of version folders backed by junctions or symbolic links ([#127](https://github.com/LiteLDev/LeviLauncher/issues/127)).
- Documented the UWP requirements, the custom WebView2 Fixed Version Runtime, and the interface material settings in English and Simplified Chinese.

### Changed

- Game versions now load mods through the game's import table.
- Refreshed the launcher interface, including navigation, package pages, and dialogs.
- Renamed the LIP content surface to Bedrinth.
- Rewrote user-facing copy in plain language and completed all eleven languages.
- Sped up launcher startup and content management.
- Bundled the required Windows runtime components with the installer.
- Licensed the project under GPL-3.0-only.
- Enforced backend tests, frontend bundle limits, and accessibility checks in the build workflow.
- Bumped version to `1.0.0`.

### Removed

- Removed the RenderDragon shader settings.
- Removed the Ctrl+R resource reload instance setting.

### Fixed

- An invalid or duplicated mod manifest can no longer overwrite an existing mod during archive import ([#114](https://github.com/LiteLDev/LeviLauncher/issues/114)).
- Generated GDK manifests now declare the runtime packages the game needs ([#121](https://github.com/LiteLDev/LeviLauncher/issues/121)).
- The update badge now opens the latest compatible LeviLamina version ([#123](https://github.com/LiteLDev/LeviLauncher/issues/123)).
- Starting a second copy now brings the running launcher to the front instead of an unrelated window with the same title.
- The launcher icon now appears on the window and in the tray.
- Downloads no longer end up in an inconsistent state.
- Settings that fail to save now report the failure instead of showing the new value.
- The world `level.dat` editor now warns before discarding unsaved edits.
- Content pages now keep selections within their own category, and a failed load stays visible instead of showing an empty list.

## [0.3.13] - 2026-06-26

### Changed

- Bumped version to `0.3.13`.

### Fixed

- Fixed a crash on Minecraft Bedrock `1.26.31` by updating the bundled `vcruntime140_1.dll`.
- Fixed Windows build workflow configuration.

## [0.3.12] - 2026-06-06

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

## [0.3.11] - 2026-05-04

### Added

- Added startup diagnostics that write `%APPDATA%\levilauncher.exe\logs\startup.log` and surface the log path when the launcher fails before the UI appears.
- Documented startup crash recovery steps and startup log reporting in the troubleshooting guides and bug report template.

### Changed

- Started WebView2 with safer software-rendered GPU settings to reduce startup crashes on affected Windows GPU and driver combinations.
- Bumped version to `0.3.11`.

### Fixed

- Normalized detected locale aliases, including script and region variants, before selecting launcher translations.
- Corrected English download page typos in version search and version totals.

## [0.3.10] - 2026-04-07

### Added

- Added German, Spanish, French, Italian, Korean, and Portuguese locale support across the launcher.(AI)
- Expanded the documentation site with German, Spanish, French, Japanese, and Korean translations.

### Changed

- Optimized the lip download experience for users in China and streamlined related launcher settings and workflows.
- Refined the update modal changelog layout for better readability.
- Cleaned up locale keys and refreshed the repository sync scripts.
- Clarified the Lanzou password in quick start guides.
- Bumped version to `0.3.10`.
- Bumped frontend dependency `picomatch` to `4.0.4`.

### Fixed

- Prevented invalid dropped file paths from breaking frontend file handling.
- Improved Visual C++ runtime detection stability.
- Shortened lip install timeouts to reduce stalled installation flows.
- Surfaced lip cache clean results directly in Settings.
- Removed the forced GDK `1.26.0.0` registration override to avoid incorrect registrations.
- Enforced exact LeviLamina-to-game-version mapping during install and backup compatibility checks.
- Waited for the frontend dev server before launching the desktop app in local development tasks.
- Restored monospace font fallbacks to avoid incorrect font rendering in the frontend.
- Fixed broken links in the Russian documentation.

## [0.3.8] - 2026-03-18

### Added

- Added an experimental instance backup and restore workflow in instance settings, including backup scope selection and restore conflict handling.
- Expanded the documentation site with Russian and Traditional Chinese (Hong Kong) translations and updated the multilingual docs configuration.

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

## [0.3.7] - 2026-03-11

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

## [0.3.6] - 2026-02-25

### Fixed

- Disabled launcher zoom gestures on touch devices to prevent accidental UI scaling.

## [0.3.5] - 2026-02-25

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
- Bumped version to `0.3.5`.

### Fixed

- Prevented launch when cancel is clicked in the force-start dialog.
- Enforced minimum window size after restore and synchronized the maximize icon state.
- Kept background image size stable when toggling blur.
- Resolved desktop path from `User Shell Folders` for shortcut handling.
- Showed unregister progress modal on the home page.
- Removed hover haze on option rows.
- Unified content management page control styles.

[Unreleased]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.5...HEAD
[1.0.5]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.13...v1.0.0
[0.3.13]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.12...v0.3.13
[0.3.12]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.11...v0.3.12
[0.3.11]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.10...v0.3.11
[0.3.10]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.8...v0.3.10
[0.3.8]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.7...v0.3.8
[0.3.7]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.6...v0.3.7
[0.3.6]: https://github.com/LiteLDev/LeviLauncher/compare/v0.3.5...v0.3.6
[0.3.5]: https://github.com/LiteLDev/LeviLauncher/releases/tag/v0.3.5
