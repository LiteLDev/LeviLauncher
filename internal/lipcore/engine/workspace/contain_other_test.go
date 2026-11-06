//go:build !windows

package workspace

import (
	"os"
	"testing"
)

// makeDirLink makes link a symbolic link to the directory target.
func makeDirLink(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
