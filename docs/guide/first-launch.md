# First Launch

The first time you open LeviLauncher, the app makes sure your device and Minecraft are ready for managed versions.

## What to expect

=== "Windows"

    On first launch, LeviLauncher may:

    - check whether Gaming Services is available (GDK)
    - check whether GameInput is available (GDK)
    - check whether the Visual C++ runtime needed by the native mod loader is installed
    - verify access to the Minecraft Bedrock (GDK / UWP) environment
    - show onboarding guidance when required components are missing

=== "Android"

    On first launch, LeviLauncher may:

    - ask you to accept the launcher agreement
    - request the permissions it needs
    - check that the official Google Play Minecraft (1.21.80 or later) is installed
    - migrate data directories left by an older launcher version

## Typical first-launch flow

=== "Windows"

    1. Open LeviLauncher.
    2. Read any setup prompt shown by the launcher.
    3. Install or repair missing prerequisites.
    4. Return to the launcher and re-check status.
    5. Continue to the **Download** page once requirements are satisfied.

=== "Android"

    1. Open LeviLauncher.
    2. Accept the agreement and grant the requested permissions.
    3. If the launcher migrates older data, wait until it finishes. Do not delete the old directory until your worlds, packs and settings are available again.
    4. Continue to the version screen.

## If a component is missing <Badge platform="windows" />

### Gaming Services

If Gaming Services is missing or broken, LeviLauncher may direct you to Microsoft Store or ask you to repair the component.

### GameInput

If GameInput is missing, install the required redistributable when the launcher prompts you.

### Minecraft Bedrock not detected

Make sure you:

- own the game on the current Microsoft account
- have installed it from Microsoft Store on this PC
- are not trying to use LeviLauncher as a replacement for the official license flow

## Good defaults for new users

- use one isolated Minecraft Release version first
- keep download and content paths on a writable drive
- avoid importing mods until your first clean launch succeeds

## After first launch

Continue with:

- [Version Management](./version-management)
- [Content Management](./content-management)
- [Settings & Personalization](./settings-personalization)
