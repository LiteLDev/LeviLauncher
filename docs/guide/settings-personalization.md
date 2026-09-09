# Settings & Personalization

LeviLauncher includes a large settings area so you can tune paths, appearance, component management, privacy choices, and update behavior.

## Main settings sections

The app groups settings into several sections, including:

- General
- Personalization
- Components
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

Open **Personalization → Background image → Interface material** to preview the current wallpaper behind content. Navigation, cards, tabs, form controls and popup surfaces share one material system. Light and dark profiles are saved independently and follow the app theme. The light/dark buttons in the material editor only select the profile to edit and preview.

| Setting | Behavior |
| --- | --- |
| Balanced | Soft glass with readability protection enabled by default |
| Clear | More visible wallpaper, with readability protection turned off |
| Solid | Opaque cards without card blur; wallpaper remains visible outside cards |
| Interface opacity | Changes neutral interface surface tints without fading text, icons or buttons |
| Interface blur | Controls blur behind panels, navigation and popup surfaces; image blur affects the entire wallpaper |
| Wallpaper overlay | Applies a light or dark tint separately from image brightness and opacity |
| Readability protection | Applies a conservative minimum tint based on theme, brightness, image opacity and blur, and strengthens secondary text |

The live preview shares the loaded image and rendering parameters with the main window. When protection increases the effective card opacity, the editor displays the applied value. This calculation does not analyze or upload the image. Reset affects only the material profile being edited.

Use **Next image** when the folder contains multiple pictures. Sequential order uses natural filename sorting; random order avoids repeating the current image. Missing, invalid or invisible images restore ordinary surfaces. Higher contrast, reduced transparency and unsupported backdrop filters use solid fallback surfaces.

### Components

- GDK-related component status
- LIP status or updates
- other launcher-managed support resources

### Privacy

- analytics-related choices if offered by the app
- external privacy or terms links

### Updates

- update checks
- beta update channel preferences

## Recommended defaults

- keep managed files on a writable drive with enough free space
- start with the stable update channel unless you want Beta behavior
- change appearance only after your base setup works well

## When to review settings

Review settings whenever you:

- move your installation to another drive
- see write permission errors
- want separate personalization per machine
- need to troubleshoot downloads or component detection
