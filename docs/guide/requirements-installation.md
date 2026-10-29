# Requirements & Installation

This page explains what LeviLauncher needs before it can install and launch Minecraft Bedrock Edition, on Windows and on Android.

UWP versions require Windows Developer Mode and the UWP framework dependencies declared by the package (such as Microsoft.VCLibs). The Gaming Services and GameInput requirements below apply to GDK. Instance isolation works for GDK and for UWP x64 packages of version 1.19.70.2 or later; non-isolated UWP instances share the Windows app's `LocalState` content. UWP x64 instances also support the console and compatible `preload-native` DLL mods, but automatic LeviLamina installation is available only for supported GDK versions.

## System requirements

=== "Windows"

    | Item | Requirement |
    | --- | --- |
    | Operating system | Windows 10 or Windows 11 |
    | Game edition | Minecraft Bedrock Edition (GDK / UWP) |
    | License | A legitimate licensed copy tied to your Microsoft account |
    | Network | Needed for downloads, version metadata, mirror tests, and update checks |

=== "Android"

    | Item | Requirement |
    | --- | --- |
    | Operating system | Android 9.0 (API 28) or later |
    | Architecture | ARM64 (arm64-v8a) |
    | Memory | 1 GB of available RAM (3 GB or more recommended) |
    | Storage | 2 GB free for Minecraft and game data (5 GB or more recommended) |
    | Game | Minecraft Bedrock Edition 1.21.80 or later, installed from Google Play |
    | License | A legitimate copy of Minecraft bought on Google Play |

## Required Windows components <Badge platform="windows" />

LeviLauncher may guide you through missing components during first launch or before installation.

- **Microsoft Gaming Services** (GDK)
- **Microsoft GameInput** (GDK)
- **WebView2 Runtime**
- **Microsoft Visual C++ Redistributable (x64)**, needed by the native mod loader; LeviLauncher checks for it and offers to install it

Depending on your Windows setup, you may already have some of them installed.

### Using a custom WebView2 Fixed Version Runtime

If the Evergreen WebView2 Runtime is not installed but you manage a WebView2 Fixed Version Runtime yourself, start LeviLauncher using either of the following methods.

Specify the directory on the command line:

```powershell
LeviLauncher.exe --webview2-runtime-dir "D:\WebView2\FixedRuntime"
```

Alternatively, create `webview2.json` next to `LeviLauncher.exe`:

```json
{
  "browserExecutableFolder": "D:\\WebView2\\FixedRuntime"
}
```

Relative paths are resolved from the directory containing `webview2.json`. The selected directory must directly contain `msedgewebview2.exe`. The command-line option takes precedence over the configuration file. If neither is configured, LeviLauncher continues to use the system-installed WebView2 Runtime.

## Before you install a version

=== "Windows"

    Complete this checklist first:

    1. Install Minecraft Bedrock from Microsoft Store at least once.
    2. Start the game once if Store installation or Gaming Services still looks incomplete.
    3. Close the game before using LeviLauncher to install or manage versions.

=== "Android"

    1. Install Minecraft Bedrock Edition from Google Play on the device.
    2. Update it to 1.21.80 or later: older versions are rejected by the launcher.
    3. Make sure there is enough free storage for the versions and worlds you plan to keep.

## Installing LeviLauncher itself

=== "Windows"

    ### Option A: GitHub Releases page

    Use this if you want the official LeviLauncher download page together with release notes and changelog context.

    1. Open the [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) page for LeviLauncher.
    2. Download the installer.
    3. Run it and finish the setup wizard.

    ### Option B: Lanzou mirror

    Use this if GitHub download speed is poor in your region.

    1. Open [Lanzou Cloud](https://levimc.lanzoue.com/b016ke39hc).
    2. Use password `levi`.
    3. Download the installer and run it locally.

=== "Android"

    1. Open the [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) page of the Android version.
    2. Download the latest APK.
    3. Allow installation from the browser or file manager you used to download it.
    4. Install the APK and open it (the app is shown as "LeviLauncher").
    5. Accept the launcher agreement and grant the permissions the app requests.

## Installing your first managed version

=== "Windows"

    1. Open **Download** inside LeviLauncher.
    2. Choose **Release** or **Preview** for the Minecraft build you want, and **GDK** or **UWP** for the package type.
    3. Pick a version entry.
    4. Decide whether to enable isolation.
    5. Start installation and wait for completion.

=== "Android"

    1. Open the version (instance) screen.
    2. Select the installed Google Play version, or import an official Minecraft APK.
    3. Enable version isolation for an imported version that should keep its data separate.
    4. Return to the launch screen and press **Launch**.

## Recommended installation strategy <Badge platform="windows" />

### Use a Minecraft Release version when

- you want the most stable day-to-day setup
- you are building a long-term world
- you want fewer surprises with mods and packs

### Use a Minecraft Preview version when

- you want to test upcoming features
- you are comfortable with instability or compatibility breaks
- you intentionally keep Minecraft Preview content separate from your main play setup

::: tip Recommended for most players
Start with one isolated **Minecraft Release** version. Add a **Minecraft Preview** version later only if you specifically need it.
:::

## If installation cannot continue

=== "Windows"

    Go to [Update & Troubleshooting](./update-troubleshooting) if you run into:

    - missing Gaming Services
    - missing GameInput
    - missing Store ownership or installation state
    - write permission problems
    - failed downloads or mirror issues

=== "Android"

    - If the launcher reports that Minecraft is missing or unsupported, install or update the Google Play version first.
    - Versions older than 1.21.80 are not supported.
    - See [Update & Troubleshooting](./update-troubleshooting) for launch failures.
