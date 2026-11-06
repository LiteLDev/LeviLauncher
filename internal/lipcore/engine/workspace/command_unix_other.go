//go:build unix && !linux

package workspace

import "errors"

// waitExited is unsupported here: Go exposes no waitid on these systems.
func waitExited(int) error { return errors.ErrUnsupported }
