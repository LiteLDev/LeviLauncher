//go:build windows

package nativeinstall

import (
	"context"
	"github.com/liteldev/LeviLauncher/internal/xbox"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedAccountRestoresCurrentPreference(t *testing.T) {
	defer xbox.ConfigureAccountSelection("", nil)
	dir := t.TempDir()
	if e := saveProtected(filepath.Join(dir, "wam-account.dpapi"), wamAccountSelection{AccountID: "selected-account"}); e != nil {
		t.Fatal(e)
	}
	if e := RestoreSharedAccount(context.Background(), dir); e != nil {
		t.Fatal(e)
	}
}
func TestSharedAccountRejectsCorruptPreference(t *testing.T) {
	defer xbox.ConfigureAccountSelection("", nil)
	dir := t.TempDir()
	p := filepath.Join(dir, "wam-account.dpapi")
	if e := os.WriteFile(p, []byte("corrupt selection"), 0600); e != nil {
		t.Fatal(e)
	}
	if RestoreSharedAccount(context.Background(), dir) == nil {
		t.Fatal("corrupt selection accepted")
	}
}
func TestSharedAccountFirstUseWithoutPreference(t *testing.T) {
	defer xbox.ConfigureAccountSelection("", nil)
	dir := t.TempDir()
	if e := RestoreSharedAccount(context.Background(), dir); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	if len(entries) != 0 {
		t.Fatal("first use created an implicit account preference")
	}
}
