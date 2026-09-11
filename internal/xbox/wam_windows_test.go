//go:build windows

package xbox

import (
	"context"
	"errors"
	"github.com/liteldev/LeviLauncher/internal/xbox/winrt/core"
	"testing"
	"time"
)

func TestLicenseCheckRejectsStaleProfile(t *testing.T) {
	ConfigureAccountSelection("selected", nil)
	defer ConfigureAccountSelection("", nil)
	cached = &session{xuid: "current", accountID: "selected", exp: time.Now().Add(time.Hour)}
	token, reference, err := GetStoreTicketForUser(context.Background(), "previous")
	if !errors.Is(err, ErrAccountChanged) || token != "" || reference != "" {
		t.Fatal("stale profile was allowed to request a Store ticket")
	}
}

func TestWAMRejectsDifferentOrMissingIdentity(t *testing.T) {
	for _, tc := range []struct {
		expected, actual, token string
		err                     error
	}{
		{"chosen", "other", "token", ErrAccountChanged},
		{"chosen", "", "token", ErrAuthenticationFailed},
		{"chosen", "chosen", "", ErrAuthenticationFailed},
		{"chosen", "chosen", "token", nil},
		{"", "default", "token", nil},
	} {
		if err := validateWAMIdentity(tc.expected, tc.actual, tc.token); !errors.Is(err, tc.err) {
			t.Fatalf("identity validation: %v", err)
		}
	}
}

func TestWAMStatusErrors(t *testing.T) {
	for _, tc := range []struct {
		status core.WebTokenRequestStatus
		err    error
	}{
		{core.WebTokenRequestStatusSuccess, nil},
		{core.WebTokenRequestStatusUserCancel, ErrInteractionRequired},
		{core.WebTokenRequestStatusUserInteractionRequired, ErrInteractionRequired},
		{core.WebTokenRequestStatusAccountProviderNotAvailable, ErrInteractionRequired},
		{core.WebTokenRequestStatusAccountSwitch, ErrAccountChanged},
		{core.WebTokenRequestStatusProviderError, ErrAuthenticationFailed},
		{core.WebTokenRequestStatus(99), ErrAuthenticationFailed},
	} {
		if err := wamStatusError(tc.status); !errors.Is(err, tc.err) {
			t.Fatalf("status %d: %v", tc.status, err)
		}
	}
}

func TestStoreTicketCancellationWhileSessionBusy(t *testing.T) {
	sessionGate <- struct{}{}
	defer func() { <-sessionGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	token, id, err := GetStoreTicket(ctx)
	if !errors.Is(err, context.Canceled) || token != "" || id != "" {
		t.Fatal("canceled request did not terminate cleanly")
	}
}

func TestRefreshPreservesPreferredAccount(t *testing.T) {
	ConfigureAccountSelection("chosen-account", nil)
	defer ConfigureAccountSelection("", nil)
	cached = &session{accountID: "chosen-account"}
	resetCache()
	if cached != nil || preferredAccountID != "chosen-account" {
		t.Fatal("refresh replaced the chosen account")
	}
}
func TestAccountSelectionFailureBlocksDefaultFallback(t *testing.T) {
	ConfigureAccountSelection("", ErrAuthenticationFailed)
	defer ConfigureAccountSelection("", nil)
	_, err := ensureSessionLocked(context.Background())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatal("failed selection attempted default authentication")
	}
}

func TestCanceledSignInPreservesAccount(t *testing.T) {
	ConfigureAccountSelection("chosen-account", nil)
	defer ConfigureAccountSelection("", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := SignIn(ctx, 1, func(string) error { t.Fatal("canceled sign-in persisted an account"); return nil }, func(func()) { t.Fatal("canceled sign-in opened a picker") })
	if !errors.Is(err, context.Canceled) || preferredAccountID != "chosen-account" {
		t.Fatal("canceled sign-in changed identity")
	}
}

func TestCanceledPreferenceRestoreDoesNotWaitForSignIn(t *testing.T) {
	sessionGate <- struct{}{}
	defer func() { <-sessionGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RestoreAccountSelection(ctx, func() (string, error) { t.Fatal("canceled restore read preference"); return "", nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("restore ignored cancellation")
	}
}
