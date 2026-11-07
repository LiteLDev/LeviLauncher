//go:build !windows

package gitproto

import "os"

// createDeleteOnCloseFile creates <prefix><random><suffix> in dir and
// unlinks it at once, so it disappears when closed, like lip's
// FileOptions.DeleteOnClose.
func createDeleteOnCloseFile(dir, prefix, suffix string) (*os.File, error) {
	f, err := os.CreateTemp(dir, prefix+"*"+suffix)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(f.Name())
	return f, nil
}
