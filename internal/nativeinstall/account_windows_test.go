//go:build windows

package nativeinstall

import (
	"context"
	"errors"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/xbox"
	"testing"
)

func TestSharedAccountErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"interaction", xbox.ErrInteractionRequired, "ERR_AUTH_INTERACTION_REQUIRED"},
		{"changed", xbox.ErrAccountChanged, "ERR_AUTH_ACCOUNT_CHANGED"},
		{"canceled", context.Canceled, "ERR_CANCELED"},
		{"deadline", context.DeadlineExceeded, "ERR_AUTH_TIMEOUT"},
		{"provider", errors.New("private provider details"), "ERR_AUTH_FAILED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := caught(func() {
				acquireUserTicket(context.Background(), func(context.Context) (string, string, error) { return "", "", fmt.Errorf("wrapped: %w", tc.err) })
			})
			var typed *Error
			if !errors.As(failure, &typed) || typed.Code != tc.code {
				t.Fatalf("wrong error mapping: %v", failure)
			}
			if typed.Reason != "launcher account authorization failed" {
				t.Fatal("provider details escaped")
			}
			if !errors.Is(failure, tc.err) {
				t.Fatal("cancellation/error cause lost")
			}
		})
	}
}

func TestSharedAccountRequiresCompleteCredential(t *testing.T) {
	defer clearOperation()
	for _, pair := range [][2]string{{"", "account"}, {"ticket", ""}} {
		if caught(func() {
			acquireUserTicket(context.Background(), func(context.Context) (string, string, error) { return pair[0], pair[1], nil })
		}) == nil {
			t.Fatal("accepted incomplete credential")
		}
	}
	acquireUserTicket(context.Background(), func(context.Context) (string, string, error) { return "ticket", "account", nil })
	if userTicket != "ticket" || userReference != "account" {
		t.Fatal("credential not available to license request")
	}
	clearOperation()
	if userTicket != "" || userReference != "" {
		t.Fatal("credential retained after operation")
	}
}

func TestDeviceOperationCancellationWhileBusy(t *testing.T) {
	installGate <- struct{}{}
	defer func() { <-installGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := accountOperation(ctx, t.TempDir(), func() { t.Fatal("canceled operation ran") })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestSharedAccountCanceledAfterTokenResponse(t *testing.T) {
	defer clearOperation()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := caught(func() {
		acquireUserTicket(ctx, func(context.Context) (string, string, error) { cancel(); return "ticket", "account", nil })
	})
	if !errors.Is(failure, context.Canceled) || userTicket != "" || userReference != "" {
		t.Fatal("canceled response was retained")
	}
}
