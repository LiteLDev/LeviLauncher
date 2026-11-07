//go:build !windows

package fsx

// SetHidden gives path the hidden attribute, best effort.
func SetHidden(string) {}
