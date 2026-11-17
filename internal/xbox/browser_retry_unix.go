//go:build darwin || linux

package xbox

import (
	"context"
	"errors"
	"io"
	"net"
)

// Device-code polling is repeatable. A dropped proxy connection must not
// invalidate a challenge the user is still approving in their browser.
func browserPollMayRetry(status int, err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if status == 502 || status == 503 || status == 504 {
		return true
	}
	var network *net.OpError
	var timeout net.Error
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.As(err, &network) || (errors.As(err, &timeout) && timeout.Timeout())
}
