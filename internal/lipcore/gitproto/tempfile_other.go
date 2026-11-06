//go:build !windows

package gitproto

import "os"

// createDeleteOnCloseTemp creates lip-*.pack in the temp directory and
// unlinks it at once, so it disappears when closed, like lip's
// FileOptions.DeleteOnClose.
func createDeleteOnCloseTemp() (*os.File, error) {
	f, err := os.CreateTemp("", "lip-*.pack")
	if err != nil {
		return nil, err
	}
	_ = os.Remove(f.Name())
	return f, nil
}
