# Update & Troubleshooting

Use this page when LeviLauncher installs, updates, or launches do not behave as expected.

## Common issues

### Gaming Services is missing or broken

- Repair or install it through the guided flow.
- If needed, revisit the Microsoft Store installation state for Minecraft Bedrock.

### GameInput is missing

- Install the required redistributable when LeviLauncher prompts you.
- Restart the launcher after installation if needed.

### Installation path is not writable

- Move your managed content path to a writable location in **Settings**.
- If the launcher requests elevation for install or self-update, allow it when you trust the action.

### A version fails to launch

- test the same version without mods
- confirm required Windows components still exist
- verify that the version finished installing correctly
- retry with a clean isolated test setup

### Downloads are too slow or fail repeatedly

- switch mirrors
- retry later
- use a local package source if available

## Safe recovery steps

1. Back up important worlds.
2. Remove recent mods or packs.
3. Test with a clean isolated version.
4. Reinstall the affected version if necessary.

## Self-update behavior

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

