//go:build !windows

package install

// shortName: there are no 8.3 short names outside Windows.
func shortName(string) string { return "" }
