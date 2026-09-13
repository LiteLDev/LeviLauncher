//go:build windows

package xbox

import (
	"context"
	"errors"
	"testing"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
)

func TestFirstUseRequiresExplicitAccountSelection(t *testing.T) {
	ConfigureAccountSelection("", nil)
	defer ConfigureAccountSelection("", nil)
	if _, err := ensureSession(); !errors.Is(err, ErrInteractionRequired) {
		t.Fatalf("first use must not authenticate the Windows default account: %v", err)
	}
	if cached != nil || preferredAccountID != "" {
		t.Fatal("first use selected an account implicitly")
	}
}

func TestSignInTransactionPreservesPreviousAccount(t *testing.T) {
	storageError := errors.New("storage unavailable")
	for _, scenario := range []string{"picker canceled", "authentication failed", "canceled after authentication", "save failed"} {
		t.Run(scenario, func(t *testing.T) {
			ConfigureAccountSelection("original", nil)
			defer ConfigureAccountSelection("", nil)
			original := &session{accountID: "original", xuid: "original-xuid", exp: time.Now().Add(time.Hour)}
			cached = original
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			saveCalls := 0
			want := ErrAuthenticationFailed
			if scenario == "picker canceled" || scenario == "canceled after authentication" {
				want = context.Canceled
			} else if scenario == "save failed" {
				want = storageError
			}
			err := signInSession(ctx, func() (*session, error) {
				if scenario == "picker canceled" || scenario == "authentication failed" {
					return nil, want
				}
				if scenario == "canceled after authentication" {
					cancel()
				}
				return &session{accountID: "chosen", xuid: "chosen-xuid"}, nil
			}, func(id string) error {
				saveCalls++
				if scenario != "save failed" || id != "chosen" {
					t.Fatal("failed sign-in tried to persist an account")
				}
				return storageError
			})
			if !errors.Is(err, want) || cached != original || preferredAccountID != "original" {
				t.Fatalf("failed sign-in replaced the previous shared account: %v", err)
			}
			if scenario == "save failed" && saveCalls != 1 {
				t.Fatal("persistence failure was not exercised")
			}
		})
	}
}

func TestSignInCommitsSelectedAccountAfterPersistence(t *testing.T) {
	ConfigureAccountSelection("original", ErrAuthenticationFailed)
	defer ConfigureAccountSelection("", nil)
	chosen := &session{accountID: "chosen", xuid: "chosen-xuid"}
	err := signInSession(context.Background(), func() (*session, error) { return chosen, nil }, func(id string) error {
		if id != "chosen" || preferredAccountID != "original" || cached != nil {
			t.Fatal("shared account changed before persistence succeeded")
		}
		return nil
	})
	if err != nil || cached != chosen || preferredAccountID != "chosen" || accountSelectionError != nil {
		t.Fatalf("successful sign-in did not publish selected identity: %v", err)
	}
}

func TestNativePickerSelectionOwnership(t *testing.T) {
	newProvider := func() *ole.IUnknown {
		return newWAMDelegate(wamProviderCmdIID, true, func(unsafe.Pointer, unsafe.Pointer) uintptr { return ole.S_OK })
	}
	isRetained := func(provider *ole.IUnknown) bool {
		wamDelegates.Lock()
		defer wamDelegates.Unlock()
		_, ok := wamDelegates.live[(*wamDelegate)(unsafe.Pointer(provider))]
		return ok
	}
	t.Run("success transfers exact callback provider", func(t *testing.T) {
		p := &wamAccountPane{}
		provider := newProvider()
		p.choose(provider)
		selected := p.takeSelection()
		p.close()
		if selected != provider || !isRetained(selected) {
			t.Fatal("picker lost the callback provider or its native selection context")
		}
		selected.Release()
	})
	t.Run("cancel and late callbacks release references", func(t *testing.T) {
		p := &wamAccountPane{}
		provider := newProvider()
		p.choose(provider)
		p.close()
		p.close()
		late := newProvider()
		p.choose(late)
		if isRetained(provider) || isRetained(late) {
			t.Fatal("canceled picker retained provider references")
		}
	})
}
