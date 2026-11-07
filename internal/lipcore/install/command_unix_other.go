//go:build unix && !linux

package install

import "errors"

// waitExitedUnreaped is unsupported here: Go exposes no waitid on these
// systems.
func waitExitedUnreaped(int) error { return errors.ErrUnsupported }
