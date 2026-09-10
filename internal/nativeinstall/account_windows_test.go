//go:build windows

package nativeinstall

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoginContinuationOrigin(t *testing.T) {
	if !IsLoginURL(LoginURL) {
		t.Fatal("initial URL rejected")
	}
	for _, raw := range []string{"http://login.live.com/x", "https://login.live.com.evil.test/x", "https://login.live.com:444/x", "https://user@login.live.com/x", "file:///x", "https://login.live.com/x#fragment"} {
		if IsLoginURL(raw) {
			t.Errorf("accepted %s", raw)
		}
	}
}
func TestProtectedAccountCorruptionDoesNotDeleteState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.dpapi")
	if err := os.WriteFile(path, []byte("invalid dpapi"), 0600); err != nil {
		t.Fatal(err)
	}
	account, err := RestoreAccount(context.Background(), dir)
	if err == nil || account.SignedIn {
		t.Fatal("corrupt account restored")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("corrupt state should be preserved until explicit sign-out")
	}
	if err := SignOut(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	account, err = RestoreAccount(context.Background(), dir)
	if err != nil || account.SignedIn {
		t.Fatal("signed out state did not restore")
	}
}
func TestCachedAccountExpiry(t *testing.T) {
	s := userSTSState{Token: "token", Username: "player@example.test", PUID: "1234", DeviceID: make([]byte, 8), ExpiresAt: time.Now().Add(time.Hour)}
	if !validUserState(s) {
		t.Fatal("valid state rejected")
	}
	s.ExpiresAt = time.Now().Add(-time.Second)
	if validUserState(s) {
		t.Fatal("expired state accepted")
	}
}
func TestAccountCancellationWhileDeviceBusy(t *testing.T) {
	installGate <- struct{}{}
	defer func() { <-installGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RestoreAccount(ctx, t.TempDir())
	if err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}
