# Changelog

All notable changes to this project will be documented in this file.

## [v0.3.10] - 2026-03-31

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

### Docs

- Clarified the Lanzou password in quick start guides.
- Fixed broken links in the Russian documentation.

### Chore

- Bumped version to `0.3.9`.

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
