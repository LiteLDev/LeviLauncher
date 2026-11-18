//go:build darwin

package xbox

import "context"

// Native Unix clients explicitly use SignInBrowser. WAM is a Windows service.
func requestWAMTicket(ctx context.Context, scope, account string) (string, string, error) {
	return "", "", ErrInteractionRequired
}
func requestWAMToken(ctx context.Context, scope, account string, hwnd uintptr, dispatch func(func())) (string, string, error) {
	return "", "", ErrInteractionRequired
}
