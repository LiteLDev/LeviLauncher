# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.1.0] - 2026-11-12

### Added

- Added a command-line mode: `LeviLauncher.exe` with a `<noun> <verb>` command (for example `play`, `install`, `instance list`, `mod install`, `mod search`, `curseforge search`) runs in the terminal and exits; without arguments the launcher window opens as before
- The LeviMC Mods page can filter by tags, license, dependencies and author, shows how many mods each choice leaves, and lists active filters as removable chips; mod pages show the author's GitHub profile, the license, the featured tags, pre-release versions and each version's dependencies. `mod search` takes the same filters with `--tag`, `--license`, `--dep` and `--author`
- Added a preset picker next to the package manager's GitHub proxy field in Settings; a custom proxy URL can still be typed in
- Launch, register, mod install/uninstall and cache-clean error dialogs show the likeliest causes in plain words, the technical detail, a copy button and a button to open the launcher logs folder

### Changed

- Replaced the external lip with a built-in lip-compatible package manager, so lip and the .NET runtime no longer need to be installed. It follows lip v0.40: packages are read from `lip.toml` and recorded in `.lip/state.json` with `side` and `platform`. Existing `tooth_lock.json` installs and older `state.json` files are migrated automatically, and packages with only a `tooth.json` keep working, including their `X#client` dependencies on packages now released with `lip.toml` (#270)
- Packages, LeviLamina included, are installed from the variant their release provides for this platform (`client`, else the default variant), and are updated and uninstalled in place; an instance never ends up with both the old `X#client` and the new `X` of the same package (#270)
- Renamed the `Third-party Components` settings tab to `Package Management`, with cache cleanup as its own item (#269)
- The LeviMC Mods page reads the new levimc.org/mods catalogue: it lists mods with a client build for this platform, searches like the site, and sorts by Relevance, Stars, Updated or Newest
- Unified the storage layout: settings live in `%APPDATA%\LeviLauncher`, while logs, accounts, instances, backups and cache live in `%LOCALAPPDATA%\LeviLauncher`; existing data is migrated once at startup without deleting anything
- The launcher updates itself from the `LeviLauncher.zip` asset of the GitHub release; with the mirror source on, it tries gh-proxy.org mirrors first and falls back to GitHub
- Nothing is fetched from jsDelivr any more, and the default GitHub proxy is now `https://gh-proxy.org/https://github.com`
- The automatic theme works out sunrise and sunset on the device instead of looking up the location by IP, and the default package icon and the RhyMC logo are bundled with the launcher
- On the install page the "Install LeviLamina" switch starts on, the default instance name is "My <version> Instance", and the installing card goes straight to the progress details
- Search, sort and filter bars of list pages and the headers of scrolling dialogs stay in place while only the list scrolls
- Renamed the close-to-tray setting to "Hide to tray on close"; closing the window while an install or update is running hides the launcher to the tray instead of quitting

### Fixed

- In GDK versions started by the launcher, in-game buttons that open a file dialog (choosing a skin, importing a world, exporting a structure) work again instead of freezing the game or doing nothing
- Installing a GDK version no longer fails over and over with `ERR_NATIVE_MSIXVC: missing device STS`; when the device token still cannot be obtained, the error is `ERR_AUTH_DEVICE_TOKEN` with the step that failed, and GDK install errors in the GUI and CLI include the reason after the code
- The package page no longer crashes with `RangeError: Invalid language tag` for languages such as `zh_CN`, and numbers, dates and times follow the launcher's UI language (#268)
- The gh-proxy.org and ghproxy.vip GitHub proxy presets no longer fail with 404, and a proxy saved in the old form is completed automatically
- Removed duplicate busy indicators and empty progress tracks, aligned simultaneous lip task progress bars, and fixed CurseForge download progress mixing in other downloads and the launcher update bar animating after the download finished
- Package and CurseForge detail pages no longer scroll when their content is short, Gaming Services is no longer checked when only UWP instances are used, and the saved window size is no longer overwritten with a minimized or maximized size
- Brought the built-in lip in line with lip's package-engine fixes for quoted script paths, preserved files, dependency versions, the `mods` folder and local paths with special characters

## [1.1.0-beta.7] - 2026-11-12

### Added

- Packages released with `tooth.json` keep working once the packages they depend on are released with `lip.toml`: on a client instance, a `tooth.json` dependency such as `LeviLamina#client` or `LegacyScriptEngine#client_lua` is looked up as `LeviLamina` or `LegacyScriptEngine#lua` when the release found is a `lip.toml` one, which no longer has `client` variants (#270). Dependencies written in a `lip.toml` are never changed: a `lip.toml` package that needs the client build of a package still released with `tooth.json` must name `X#client` itself

### Changed

- An instance never ends up with both the old `tooth.json` client build (`X#client`) and the new `lip.toml` package (`X`) of the same package: installing or updating to a `lip.toml` release replaces `X#client` in place, and moving back to a `tooth.json` release replaces `X` with `X#client` (#270)
- Installing or updating LeviLamina and catalogue mods picks `X#client` or `X` from the release being installed, and the LeviLamina version that matches the game is only moved when the move is to its `lip.toml` release (#270)
- When the catalogue cannot be reached, a mod is installed as `X` instead of `X#client` only when its release has no `client` variant, no longer after any download error (#270)

## [1.1.0-beta.6] - 2026-11-10

### Changed

- Renamed settings tab `Third-party Components` to `Package Management`, drop 'lip' wording, make cache cleanup its own item (#269)

### Fixed

- Pass BCP 47 locale tags to Intl formatting (#268)
  i18next stores languages as app codes like zh_CN, which Intl rejects with
  RangeError: Invalid language tag, crashing the LIP page. Add toIntlLocale /
  getIntlLocale and use them for every toLocale* call so numbers, dates and
  times also follow the launcher's UI language instead of the OS locale.

## [1.1.0-beta.5] - 2026-11-10

### Fixed

- Upgrading to the built-in lip that follows lip v0.40 no longer makes package operations fail with `ERR_LIP_INSTALL_FAILED` because the instance's `.lip/state.json` was written with the old keys: before the first package operation on an instance (install, update, uninstall, listing or a backup), the launcher converts `platform` to `side` and `runtime` to `platform`, in the state file and in the package files it records. Only the keys are renamed, the file is replaced atomically, and the conversion is skipped when lip could not read the result

## [1.1.0-beta.4] - 2026-11-10

### Added

- The LeviMC Mods page can filter by tags (the featured tags such as Library / API or Economy first, each with its icon; a tag can also be excluded), license (Permissive, Weak copyleft, Copyleft, Other or No license, or an exact license), dependencies and author, shows how many mods each choice leaves, and lists active filters as removable chips; a mod's page has a "More by <author>" link
- Mod pages show the author as a link to their GitHub profile, the license with its family, and the featured tags with their icons before the other tags; versions list pre-releases as such and name their dependencies
- `mod search` in the command line takes `--tag`, `--license`, `--dep` and `--author`, and its JSON output includes each mod's owner, license, license family, dependencies and game versions

### Changed

- The built-in package manager follows lip v0.40: an entry of `lip.toml` and `.lip/state.json` says where it runs with `side` (`server` or `client`) and `platform` (a runtime identifier such as `win-x64`), formerly `platform` and `runtime`. A state file with the old keys is refused with a message saying to rename them
- The LeviMC Mods page reads the new levimc.org/mods catalogue, from its JSON at `levimc.org/api/v4/mods/` (`levimc.org/mods/*.json` keeps the old JSON for earlier versions): it lists the mods with a client build for this computer's platform, searches like the site (every word must match, best matches first, in English or Chinese tag names too) and sorts by Relevance, Stars, Updated or Newest; the Name sort, the order switch and "Hotness" are gone. The game version filter uses the game versions the catalogue lists, so it works without the LeviLamina version list
- Installing a mod without choosing a variant installs its "client" variant, else its default variant, else the first other variant with a client build for this platform, as the catalogue says; an installed mod is still updated in place whatever its variant
- The developer guide on the LeviMC Mods page explains `side` and `platform` in `lip.toml` and links to lip's guide to creating packages
- `mod search --sort` takes `relevance` (the default), `stars`, `updated` or `newest`; `--sort name` and `--reverse` are gone, and `--game` matches the catalogue's game versions. The command line caches the catalogue in `lip-catalog.v4.json`
- The launcher updates itself from the `LeviLauncher.zip` asset of the GitHub release instead of the GoProxy module mirrors; with the mirror source on, it tries gh-proxy.org mirrors first and falls back to GitHub
- The default GitHub proxy of the package manager and `config` is now `https://gh-proxy.org/https://github.com` (formerly `https://github.bibk.top`), and `gh.bibk.top` is added to the presets
- The GDK version list, the LeviLamina version list and mod READMEs are no longer fetched from jsDelivr: they come from GitHub, through gh-proxy.org or gh.bibk.top first only when the mirror source is on

### Fixed

- The gh-proxy.org and ghproxy.vip presets of the package manager's GitHub proxy no longer fail with 404: they now end in `/https://github.com` (for example `https://gh-proxy.org/https://github.com`), and a proxy saved in the old form is completed automatically. `v4.gh-proxy.org` is added to the presets

## [1.1.0-beta.3] - 2026-11-09

### Changed

- The "Install LeviLamina" switch on the install page now starts on; it only takes effect when the version supports LeviLamina
- The default instance name is now "My <version> Instance", translated into the UI language
- The installing card no longer repeats the page title and a generic hint, and starts directly with the version, target and progress details

### Fixed

- In GDK versions started by the launcher, in-game buttons that open a file dialog (choosing a skin, importing a world, exporting a structure) open the dialog again instead of freezing the game or doing nothing

## [1.1.0-beta.2] - 2026-11-09

### Added

- Added a preset picker next to the LIP GitHub proxy field in Settings; a custom proxy URL can still be typed in
- Launch, register, mod install/uninstall and cache-clean error dialogs now show the likeliest causes in plain words, the technical detail of the failure, a copy button and a button to open the launcher logs folder; mod action toasts include the first cause

### Changed

- Kept the search, sort and filter bars of the resource pack, behavior pack, world, skin pack, server, screenshot and download task pages in place while only the list scrolls; scrolling dialogs (update changelog, process management, mirror selection, lip task console) likewise keep their headers and toolbars fixed
- Renamed the close-to-tray setting to "Hide to tray on close" and clarified that the minimize button still minimizes to the taskbar
- Closing the window from the taskbar or with Alt+F4 while an install or update is running now hides the launcher to the tray instead of quitting
- The automatic theme now works out sunrise and sunset on the device from the system time zone, instead of looking up the location by IP through `ip-api.com` and `sunrise-sunset.org`
- The default package icon and the RhyMC logo on the About page are bundled with the launcher instead of being loaded from GitHub and rhymc.com

### Fixed

- Removed duplicate busy indicators where a progress bar is already shown (launch dialog, lip cache cleaning, LeviLamina and lip package install dialogs, install page), and showed an indeterminate bar instead of an empty track or nothing before the first progress update
- Aligned simultaneously shown lip task progress bars and their percentages, and gave the install page's extraction bar the same style as the LeviLamina bar that follows it
- CurseForge downloads no longer show progress from other downloads running at the same time, or an invalid value when the file size is unknown
- The launcher update's download bar no longer starts animating again after the download has finished
- Detail pages for lip packages and CurseForge mods no longer scroll when their content is short
- Gaming Services is no longer checked at startup when only UWP instances are used
- The saved window size is no longer overwritten with a minimized or maximized size on close
- Installing a GDK version no longer fails over and over with `ERR_NATIVE_MSIXVC: missing device STS` when Microsoft stops accepting the launcher's registered device: the launcher registers a new device once and retries, keeping the old credential as `device.dpapi.rejected` in the accounts folder
- When Microsoft still does not issue the device token, the install error is now `ERR_AUTH_DEVICE_TOKEN` and shows which step failed with its HTTP status and fault codes, or that login.live.com could not be reached or was answered by something else (such as a proxy), instead of suggesting a damaged download
- GDK install errors in the GUI and CLI now include the failure reason after the error code
- Brought the built-in lip in line with lip's latest package-engine fixes: package scripts are passed to `cmd.exe` as written, so quoted paths work; whether a preserved file is kept, and what uninstalling removes, is decided after the package's scripts run; installed dependencies keep their versions when they still fit; updating without arguments handles sources that now give another package; the `mods` folder is only created when an operation changes files; and local paths with `%`, `#` or non-ASCII characters are recorded correctly

## [1.1.0-beta.1] - 2026-11-08

### Added

- Added a command-line mode: `LeviLauncher.exe` with a `<noun> <verb>` command (for example `play`, `install`, `instance list`, `mod install`, `curseforge search`) runs in the terminal and exits; without arguments the launcher window opens as before

### Changed

- Unified the launcher's storage layout and naming: settings live in `%APPDATA%\LeviLauncher`, while logs, accounts, instances, backups and cache live in `%LOCALAPPDATA%\LeviLauncher`; existing data is migrated once at startup without deleting anything, and the launcher keeps using the old layout until the next start if an instance is in use or a custom data folder is offline
- Replaced the external lip package manager with a built-in lip-compatible package manager, so lip and the .NET runtime no longer need to be installed or updated from Settings; existing `tooth_lock.json` installs keep working
- Aligned the built-in lip with the files of lip's format 4: packages are recorded in `.lip/state.json` and read from `lip.toml` (packages with only a `tooth.json` still work), existing `tooth_lock.json` installs are migrated automatically on the first package operation, and the GitHub proxy is read from lip's `config.toml` (a `liprc.json` is converted)
- Packages, LeviLamina included, are installed from their `client` variant, or from their default variant when they have none; installed packages are updated and uninstalled in place

## [1.0.7] - 2026-11-04

### Added

- Added mirror source switch in settings

### Changed

- Switched npm mirror to taobao mirror

## [1.0.6] - 2026-11-02

### Changed

- Switched the LeviMC Mods package index from `lipr.levimc.org/levilauncher.json` to `levimc.org/mods/index.json` and per-mod `levimc.org/mods/<tooth>/index.json`

### Fixed

- Fixed installed LeviLamina and lip packages showing as not installed with lip v0.35.0 and later
- Fixed the LeviLamina install state waiting for the LeviMC Mods index to load
- Fixed the GDK version list failing to load without a proxy by adding more catalog mirrors and recognizing the `zh_CN` UI language

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

[Unreleased]: https://github.com/LiteLDev/LeviLauncher/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.7...v1.1.0
[1.1.0-beta.7]: https://github.com/LiteLDev/LeviLauncher/compare/v1.1.0-beta.6...v1.1.0-beta.7
[1.1.0-beta.6]: https://github.com/LiteLDev/LeviLauncher/compare/v1.1.0-beta.5...v1.1.0-beta.6
[1.1.0-beta.5]: https://github.com/LiteLDev/LeviLauncher/compare/v1.1.0-beta.4...v1.1.0-beta.5
[1.1.0-beta.4]: https://github.com/LiteLDev/LeviLauncher/compare/v1.1.0-beta.3...v1.1.0-beta.4
[1.1.0-beta.3]: https://github.com/LiteLDev/LeviLauncher/compare/v1.1.0-beta.2...v1.1.0-beta.3
[1.1.0-beta.2]: https://github.com/LiteLDev/LeviLauncher/compare/v1.1.0-beta.1...v1.1.0-beta.2
[1.1.0-beta.1]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.7...v1.1.0-beta.1
[1.0.7]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.6...v1.0.7
[1.0.6]: https://github.com/LiteLDev/LeviLauncher/compare/v1.0.5...v1.0.6
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
