# Requirements & Installation

This page explains what LeviLauncher needs before it can install and launch Minecraft Bedrock (GDK) versions correctly.

## System requirements

| Item | Requirement |
| --- | --- |
| Operating system | Windows 10 or Windows 11 |
| Game edition | Minecraft Bedrock Edition (GDK) |
| License | A legitimate licensed copy tied to your Microsoft account |
| Network | Needed for downloads, version metadata, mirror tests, and update checks |

## Required Windows components

LeviLauncher may guide you through missing components during first launch or before installation.

- **Microsoft Gaming Services**
- **Microsoft GameInput**
- **WebView2 Runtime**

Depending on your Windows setup, you may already have some of them installed.

## Before you install a version

Complete this checklist first:

1. Install Minecraft Bedrock from Microsoft Store at least once.
2. Start the game once if Store installation or Gaming Services still looks incomplete.
3. Close the game before using LeviLauncher to install or manage versions.

## Installing LeviLauncher itself

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

## Installing your first managed version

1. Open **Download** inside LeviLauncher.
2. Choose **Release** or **Preview** for the Minecraft build you want.
3. Pick a version entry.
4. Decide whether to enable isolation.
5. Start installation and wait for completion.

## Recommended installation strategy

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

Go to [Update & Troubleshooting](./update-troubleshooting) if you run into:

- missing Gaming Services
- missing GameInput
- missing Store ownership or installation state
- write permission problems
- failed downloads or mirror issues
