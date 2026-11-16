package nativeinstall

import "context"

// ProbeDevice provisions and authenticates this application's own Store device.
// It returns no credentials or content keys to its caller.
func ProbeDevice(ctx context.Context, dir string) error {
	return accountOperation(ctx, dir, func() { ensureDevice(); _ = deviceTicket() })
}
