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

LeviLauncher includes a built-in package manager compatible with [lip](https://github.com/LiteLDev/lip), so you do not need to install lip or the .NET runtime separately. It follows lip 0.38: each version records its packages in `.lip/workspace.yaml`, and packages are read from their `lip.yaml` (older packages that only have a `tooth.json` still work). Versions that already have LeviLamina or LIP packages installed with an older launcher (`tooth_lock.json`) are migrated automatically on the first package operation. When **Use mirror sources** is on, package downloads from GitHub go through the **GitHub proxy** set in [Settings](./settings-personalization); otherwise they connect to GitHub directly, through your system proxy if one is set. A GitHub proxy you saved earlier, or a `github_proxy` set in lip's `config.yaml` (in `%APPDATA%\lip`, for example with `lip config set github_proxy <URL>`), applies either way. **Clean Cache** removes downloaded package files.

These integrations are aimed at more advanced users who want package-driven or loader-driven mod setups.

Best practices:

- prefer stable Minecraft Release versions first
- avoid changing too many moving parts at once
- note the exact version you tested successfully

To add LeviLamina to a client version, see [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/) in the LeviLamina documentation.

## Native mods <Badge platform="android" />

Native mods run native code in the game process. Install only mods from sources you trust, and test them in an isolated version first.

To make your own, follow [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) in the LeviLamina documentation.

## If something breaks after adding mods

- disable the newest mod first
- compare against the last known good version
- move important worlds to safety before more testing
- continue with [Update & Troubleshooting](./update-troubleshooting)
