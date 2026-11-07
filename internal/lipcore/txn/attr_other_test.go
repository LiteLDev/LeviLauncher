//go:build !windows

package txn

import (
	"os"
	"testing"
)

func makeReadOnly(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
}

func makeHidden(*testing.T, string) {}

func makeNormal(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
}

// attrString lists whether a file is read-only.
func attrString(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o200 == 0 {
		return "R"
	}
	return ""
}

const hiddenBackups = false
