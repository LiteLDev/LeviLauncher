//go:build darwin

package nativeinstall

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedCacheRejectsTamperingAndWrongRecord(t *testing.T) {
	original := protectedKey
	defer func() { protectedKey = original }()
	protectedKey = func(string, bool) ([]byte, error) { return bytes.Repeat([]byte{7}, 32), nil }
	dir := t.TempDir()
	path := filepath.Join(dir, "device.dpapi")
	want := ownDevice{Member: "private-member", Password: "private-password", PUID: "account", License: "license"}
	if err := saveProtected(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(want.Password)) {
		t.Fatal("plaintext secret written")
	}
	var got ownDevice
	if err = loadProtected(path, &got); err != nil || got != want {
		t.Fatalf("round trip: %v", err)
	}
	other := filepath.Join(dir, "account.dpapi")
	os.WriteFile(other, data, 0600)
	if loadProtected(other, &got) == nil {
		t.Fatal("record substitution accepted")
	}
	data[len(data)-1] ^= 1
	os.WriteFile(path, data, 0600)
	if loadProtected(path, &got) == nil {
		t.Fatal("tampered cache accepted")
	}
	os.WriteFile(path, []byte("old DPAPI blob"), 0600)
	if loadProtected(path, &got) == nil {
		t.Fatal("foreign protection format accepted")
	}
}
func TestNativeCacheLockExcludesAnotherOpen(t *testing.T) {
	dir := t.TempDir()
	lock := lockDeviceCache(dir)
	defer closeCacheLock(lock)
	var err error
	func() { defer recoverFailure(&err); second := lockDeviceCache(dir); defer closeCacheLock(second) }()
	if err == nil {
		t.Fatal("second exclusive lock acquired")
	}
}
func TestNativeDeviceChallengeShape(t *testing.T) {
	value, err := deviceInfo()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(value), []byte(`name="8196"`)) || !bytes.Contains([]byte(value), []byte(`name="8197"`)) {
		t.Fatal("missing device challenge versions")
	}
	if makeDeviceInfo(nil) != makeDeviceInfo(nil) {
		t.Fatal("unavailable hardware challenge changed")
	}
}
