# Version Management

LeviLauncher treats each installed setup as a version you can manage independently. In the app, installed versions are called instances and are listed on the **Instances** page.

## What you can do

=== "Windows"

    - install Minecraft Release or Preview builds
    - give versions recognizable names
    - launch a selected version quickly
    - remove versions you no longer need
    - open version-specific settings
    - create a desktop shortcut for a version

=== "Android"

    - import official Minecraft APKs and run them inside the launcher, without installing each version on the system
    - keep multiple versions and their data separated
    - choose the version the launch screen starts

## Minecraft Release vs Preview <Badge platform="windows" />

| Type | Best for | Notes |
| --- | --- | --- |
| Release | Stable daily play | Recommended for most players |
| Preview | Testing upcoming changes | May break worlds, packs, or mods more often |

## Isolation explained

=== "Windows"

    Isolation is available for GDK versions and for UWP x64 versions 1.19.70.2 or later. When isolation is enabled, LeviLauncher stores the managed game data inside that version's own workspace rather than letting everything share the same default data location.

    Isolation is useful when you want:

    - different content sets for different versions
    - safer testing for mods or Minecraft Preview builds
    - less risk of one setup affecting another

=== "Android"

    Enable version isolation for an imported version when its worlds, packs and settings should stay separate from the installed Google Play version. Imported versions often need isolation to launch.

## Common version tasks <Badge platform="windows" />

### Install a new version

Use the **Download** page, then return to **Launch** after installation finishes.

### Rename a version

Use **Instances** or **Instance Settings** to give the setup a name that reflects its purpose, such as `Release Survival` or `Preview Test`.

### Delete a version

Remove versions you no longer need, especially Minecraft Preview test installs that are no longer relevant.

::: warning Be careful before deleting
Deleting a version can remove the managed environment for that setup. Back up important worlds first.
:::

### Create a desktop shortcut

Use **Create Desktop Shortcut** in the menu next to the **Launch** button to create a shortcut for a specific version so you can launch it directly.

## Recommended version strategy <Badge platform="windows" />

- one isolated Minecraft Release version for normal play
- one isolated Minecraft Preview version only when needed
- one temporary test version for mods or troubleshooting

## Related guides

- [Downloads & Mirrors](./downloads-mirrors)
- [World Tools](./world-tools)
- [Update & Troubleshooting](./update-troubleshooting)
