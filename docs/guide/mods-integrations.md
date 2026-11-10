# Mods & Integrations

LeviLauncher supports mod-related workflows that go beyond a basic launcher.

## Available capabilities

=== "Windows"

    - import mod packages (`.mcmod`)
    - `.zip` packages and bare `.dll` mods are still accepted: choose **All files (\*.\*)** in the file picker
    - enable or disable installed mods
    - remove mods you no longer need
    - browse or fetch content through CurseForge-related flows
    - install LIP packages with the built-in package manager, compatible with [lip](https://github.com/LiteLDev/lip)
    - work with LeviLamina-related mod setups

    ::: warning Advanced area
    Mods, loaders, and package integrations can change compatibility quickly, especially on Minecraft Preview builds. Treat this area as advanced and test carefully.
    :::

=== "Android"

    - load native mods (`.so` libraries) into the game
    - keep modded setups in isolated versions

    Building native mods? See [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) in the LeviLamina documentation.

## Recommended mod workflow

1. Start from a clean isolated version.
2. Confirm the version launches successfully without mods.
3. Add one mod or one small group at a time.
4. Test launch after each change.
5. Keep a backup path for important worlds.

## CurseForge <Badge platform="windows" />

The launcher provides CurseForge-oriented browsing and package flows so you can discover compatible mod content more easily from inside the app.

Use it when you want:

- a more guided discovery workflow
- a clearer view of compatible files for a project
- less manual searching outside the launcher

## lip and LeviLamina <Badge platform="windows" />

LeviLauncher includes a built-in package manager compatible with [lip](https://github.com/LiteLDev/lip), so you do not need to install lip or the .NET runtime separately. It follows lip: each version records its packages in `.lip/state.json`, and packages are read from their `lip.toml` (older packages that only have a `tooth.json` still work). Versions that already have LeviLamina or LIP packages installed with an older launcher (`tooth_lock.json`) are migrated automatically on the first package operation. When **Use mirror sources** is on, package downloads from GitHub go through the **GitHub proxy** set in [Settings](./settings-personalization); otherwise they connect to GitHub directly, through your system proxy if one is set. A GitHub proxy you saved earlier, or a `github_proxy` set in lip's `config.toml` (in `%APPDATA%\lip`, for example with `lip config set github_proxy <URL>`), applies either way. **Clean Cache** removes downloaded package files.

Since lip v0.40, each entry of a `lip.toml` and of `.lip/state.json` says where it runs with `side` (`server` or `client`) and `platform` (a .NET runtime identifier such as `win-x64`); lip v0.39 and earlier called them `platform` and `runtime`. A `.lip/state.json` written with the old keys, by lip v0.39 or earlier or by an older launcher, is converted automatically before the next package operation: `platform` is renamed to `side` and `runtime` to `platform`. A file lip still cannot read is refused with a message that names the change: rename `platform` to `side` and `runtime` to `platform`, or reinstall the packages of that version.

These integrations are aimed at more advanced users who want package-driven or loader-driven mod setups.

Best practices:

- prefer stable Minecraft Release versions first
- avoid changing too many moving parts at once
- note the exact version you tested successfully

To add LeviLamina to a client version, see [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/) in the LeviLamina documentation.

### LeviMC Mods page

The **LeviMC Mods** page lists the mods of [levimc.org/mods](https://levimc.org/mods/) that have a build for the client on Windows x64 (an entry with `side = "client"` and `platform = "win-x64"`). Search them by name, id, author, tag or description; sort them by relevance, stars, last update or newest; and filter them by game version, LeviLamina version, tags (the featured tags such as Library / API or Economy come first, and a tag can also be excluded), license (a family such as Permissive, or an SPDX id), dependencies and author (**More by …** on a mod's page). Installing a mod picks its `client` variant, else its default variant, else the first other variant with a client build; a mod's page lets you choose another of these variants.

## Native mods <Badge platform="android" />

Native mods run native code in the game process. Install only mods from sources you trust, and test them in an isolated version first.

To make your own, follow [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) in the LeviLamina documentation.

## If something breaks after adding mods

- disable the newest mod first
- compare against the last known good version
- move important worlds to safety before more testing
- continue with [Update & Troubleshooting](./update-troubleshooting)
