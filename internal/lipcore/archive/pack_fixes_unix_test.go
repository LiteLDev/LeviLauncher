//go:build unix

package archive

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// makeSpecialFiles creates a FIFO and a Unix domain socket file, which
// exists until the test ends.
func makeSpecialFiles(t *testing.T, fifo, socket string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(fifo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	if !isSpecialFile(fifo) || !isSpecialFile(socket) {
		t.Fatal("not special files")
	}
}

// shortName: there are no 8.3 short names outside Windows.
func shortName(string) string { return "" }
