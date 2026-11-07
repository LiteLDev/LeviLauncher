// Package lipdirs holds lip 0.38's per-user directories
// (src/Lip.Core/Infrastructure/LipDirectories.cs): the platform's known
// folders on Windows, the XDG Base Directory specification elsewhere.
package lipdirs

import "path/filepath"

const appName = "lip"

// ConfigDir is lip's user-editable configuration directory: %APPDATA%\lip
// on Windows, $XDG_CONFIG_HOME/lip (default ~/.config/lip) elsewhere. .NET
// maps SpecialFolder.ApplicationData to both.
func ConfigDir() string { return filepath.Join(applicationData(), appName) }

// CacheDir is lip's cache (protocol/userspace.md §3), data that can be
// deleted without losing anything: %LOCALAPPDATA%\lip\Cache on Windows,
// which has no cache folder of its own, and $XDG_CACHE_HOME/lip (default
// ~/.cache/lip) elsewhere.
func CacheDir() string { return cacheDir() }

// LocalAppData mirrors .NET Environment.GetFolderPath(
// SpecialFolder.LocalApplicationData, SpecialFolderOption.DoNotVerify).
func LocalAppData() string { return localAppData() }
