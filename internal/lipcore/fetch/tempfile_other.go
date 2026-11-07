//go:build !windows

package fetch

import "os"

// createDeleteOnClose creates path (failing if it exists) for reading and
// writing and unlinks it at once, so that, as with .NET's
// FileOptions.DeleteOnClose, nothing is left once it is closed.
func createDeleteOnClose(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
