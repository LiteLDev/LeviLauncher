// Package lipdirs holds lip 0.38's per-user directories
// (src/Lip.Core/Infrastructure/LipDirectories.cs): the platform's known
// folders on Windows, the matching ~/Library folders on macOS, and the XDG
// Base Directory specification elsewhere.
//
// On macOS this departs from lip itself, which follows .NET there
// (~/.config/lip, ~/.cache/lip): the launcher keeps lip's files where macOS
// keeps every app's, as it does on Windows. The standalone lip does not see
// the launcher's lip configuration or cache on macOS.
package lipdirs

import "path/filepath"

const appName = "lip"

// ConfigDir is lip's user-editable configuration directory: %APPDATA%\lip
// on Windows, ~/Library/Application Support/lip on macOS, $XDG_CONFIG_HOME/lip
// (default ~/.config/lip) elsewhere.
func ConfigDir() string { return filepath.Join(applicationData(), appName) }

// CacheDir is lip's cache (protocol/userspace.md §3), data that can be
// deleted without losing anything: %LOCALAPPDATA%\lip\Cache on Windows,
// which has no cache folder of its own, ~/Library/Caches/lip on macOS, and
// $XDG_CACHE_HOME/lip (default ~/.cache/lip) elsewhere.
func CacheDir() string { return cacheDir() }

// LocalAppData mirrors .NET Environment.GetFolderPath(
// SpecialFolder.LocalApplicationData, SpecialFolderOption.DoNotVerify).
func LocalAppData() string { return localAppData() }
