//go:build darwin

package registry

// Windows development-mode registration is not a host feature on Unix.
func IsDevModeEnabled() bool { return false }
func TryEnableDevMode() bool { return false }
