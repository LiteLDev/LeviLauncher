---
platforms: [windows]
---
# Settings & Personalization

LeviLauncher includes a large settings area so you can tune paths, appearance, component management, privacy choices, and update behavior.

## Main settings sections

The app groups settings into several sections, including:

- Basic Settings
- Personalization
- Third-party Components
- Others
- Privacy
- Updates
- About

## What you can adjust

### Paths and storage

- base content path
- writable location for managed files
- environment placement that suits your disk layout

### Appearance

- language
- light or dark preference
- theme color behavior
- background and visual personalization options

#### Wallpaper and interface material

Choose an image folder under **Personalization → Wallpaper → Background Image**, then open **Personalization → Materials and readability** to adjust **Interface material**, the layer between the wallpaper and the content. Navigation, cards, tabs, form controls and popup surfaces share one material system. Light and dark profiles are saved separately and applied automatically with the app theme; the **Light Mode** / **Dark Mode** buttons in the material editor only select the profile to edit and preview.

| Setting | Behavior |
| --- | --- |
| Balanced | Default preset: soft glass with readability protection turned on |
| Clear | More wallpaper shows through the cards, with readability protection turned off; best with simple, low-contrast images |
| Solid | Opaque cards without card blur; wallpaper remains visible outside cards |
| Interface opacity | Changes neutral interface surface tints without fading text, icons or buttons |
| Interface blur | Controls blur behind panels, navigation and popup surfaces; image blur affects the entire wallpaper |
| Wallpaper overlay | Softens the wallpaper with a light tint in light mode and a dark tint in dark mode, separately from image brightness and opacity |
| Readability protection | Applies a conservative minimum tint based on theme, brightness, image opacity and blur, and strengthens secondary text |

The live preview shares the loaded image and rendering parameters with the main window. When protection increases the effective card opacity, the editor displays the applied value. This calculation does not analyze or upload the image. **Reset this theme** restores only the material profile being edited; it does not clear the image folder or the other theme's profile.

Use **Next image** when the folder contains multiple pictures. **Sequential** order uses natural filename sorting; **Random** order avoids repeating the current image. When no image is selected, the image cannot be loaded, or image opacity is 0, the interface returns to ordinary surfaces. Higher contrast, reduced transparency and unsupported backdrop filters use solid fallback surfaces.

### Third-party Components

- [lip](https://github.com/LiteLDev/lip) status: check, install, or update it, and clean its cache

### Privacy

- anonymous usage analytics (Microsoft Clarity), which you can turn off at any time
- links to the Clarity terms and the Microsoft privacy statement

### Updates

- update checks
- **Receive Beta Updates** to get prerelease launcher updates

## Recommended defaults

- keep managed files on a writable drive with enough free space
- leave **Receive Beta Updates** off unless you want prerelease launcher builds
- change appearance only after your base setup works well

## When to review settings

Review settings whenever you:

- move your installation to another drive
- see write permission errors
- want separate personalization per machine
- need to troubleshoot downloads or component detection
