//go:build !windows

package install

// getLongPath: there are no 8.3 short names outside Windows.
func getLongPath(path string) (string, bool) { return path, true }
