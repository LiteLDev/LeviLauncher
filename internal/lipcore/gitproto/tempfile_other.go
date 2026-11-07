//go:build !windows

package gitproto

import "os"

// createDeleteOnCloseTemp creates lip-*.pack in the temp directory and
// unlinks it at once, so it disappears when closed, like lip 0.37.0's
// FileOptions.DeleteOnClose.
func createDeleteOnCloseTemp() (*os.File, error) {
	return createDeleteOnCloseFile(os.TempDir(), "lip-", ".pack")
}

// createDeleteOnCloseFile creates <prefix><random><suffix> in dir and
// unlinks it at once, so it disappears when closed.
func createDeleteOnCloseFile(dir, prefix, suffix string) (*os.File, error) {
	f, err := os.CreateTemp(dir, prefix+"*"+suffix)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(f.Name())
	return f, nil
}
