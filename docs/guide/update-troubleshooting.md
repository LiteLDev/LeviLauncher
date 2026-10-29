# Update & Troubleshooting

Use this page when LeviLauncher installs, updates, or launches do not behave as expected.

## Common issues <Badge platform="windows" />

### Gaming Services is missing or broken

- Repair or install it through the guided flow.
- If needed, revisit the Microsoft Store installation state for Minecraft Bedrock.

### GameInput is missing

- Install the required redistributable when LeviLauncher prompts you.
- Restart the launcher after installation if needed.

### Installation path is not writable

- Move your managed content path to a writable location in **Settings → Basic Settings**.
- If the launcher requests elevation for install or self-update, allow it when you trust the action.

### LeviLauncher closes immediately or never shows a window

- On Windows, LeviLauncher now starts WebView2 in a safer software-rendered mode to reduce startup crashes on some GPU and driver combinations.
- If the launcher still fails before the UI appears, check `%APPDATA%\levilauncher.exe\logs\startup.log`.
- Retry once after a failed launch. If it still happens, attach `startup.log` when opening a GitHub issue.

### A version fails to launch

- test the same version without mods
- confirm required Windows components still exist
- verify that the version finished installing correctly
- retry with a clean isolated test setup

### Downloads are too slow or fail repeatedly

- switch mirrors
- retry later
- use a local package source if available

## Common issues on Android <Badge platform="android" />

### Minecraft is missing or unsupported

Install or update the official Minecraft app from Google Play (1.21.80 or later), then open LeviLauncher again.

### An imported version fails to launch

Imported versions often need version isolation. Enable it in the instance settings and try again.

### A launch fails

Open the launch log from the failure screen and check the reported reason before changing settings.

### Data migration

The launcher may migrate older data directories before continuing. Do not delete the old directory until the migration succeeds and your worlds, packs and settings are available.

## Safe recovery steps

1. Back up important worlds.
2. Remove recent mods or packs.
3. Test with a clean isolated version.
4. Reinstall the affected version if necessary.

## Self-update behavior <Badge platform="windows" />

LeviLauncher can check, download, and install app updates. Some environments may require elevated permission if the installation directory is not writable.

## When to report a bug
Open a GitHub issue when:

- you can reproduce the problem consistently
- the problem persists after basic recovery steps
- the issue looks specific to LeviLauncher rather than a general Windows setup problem

When reporting, include:

- Windows version
- LeviLauncher version
- the exact steps you followed
- screenshots or logs if available

Open issues here: [GitHub Issues](https://github.com/LiteLDev/LeviLauncher/issues)

For the Android version, include the Android version, the device model and the launch log, and open the issue in [LiteLDev/LeviLaunchroid](https://github.com/LiteLDev/LeviLaunchroid/issues).
