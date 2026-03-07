# Version Management

LeviLauncher treats each installed setup as a version you can manage independently.

## What you can do

- install Minecraft Release or Preview builds
- give versions recognizable names
- launch a selected version quickly
- remove versions you no longer need
- open version-specific settings
- create a desktop shortcut for a version

## Minecraft Release vs Preview

| Type | Best for | Notes |
| --- | --- | --- |
| Release | Stable daily play | Recommended for most players |
| Preview | Testing upcoming changes | May break worlds, packs, or mods more often |

## Isolation explained

When isolation is enabled, LeviLauncher stores the managed game data inside that version's own workspace rather than letting everything share the same default data location.

Isolation is useful when you want:

- different content sets for different versions
- safer testing for mods or Minecraft Preview builds
- less risk of one setup affecting another

## Common version tasks

### Install a new version

Use the **Download** page, then return to **Home** after installation finishes.

### Rename a version

Use **Versions** or the version settings page to give the setup a name that reflects its purpose, such as `Release Survival` or `Preview Test`.

### Delete a version

Remove versions you no longer need, especially Minecraft Preview test installs that are no longer relevant.

::: warning Be careful before deleting
Deleting a version can remove the managed environment for that setup. Back up important worlds first.
:::

### Create a desktop shortcut

LeviLauncher can create a desktop shortcut for a specific version so you can launch it directly.

## Recommended version strategy

- one isolated Minecraft Release version for normal play
- one isolated Minecraft Preview version only when needed
- one temporary test version for mods or troubleshooting

## Related guides

- [Downloads & Mirrors](./downloads-mirrors)
- [World Tools](./world-tools)
- [Update & Troubleshooting](./update-troubleshooting)
