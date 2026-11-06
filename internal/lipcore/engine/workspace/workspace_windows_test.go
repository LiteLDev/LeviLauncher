//go:build windows

package workspace

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestDefaultCommandRunnerNoConsoleWindow(t *testing.T) {
	var flags uint32
	orig := startCommand
	t.Cleanup(func() { startCommand = orig })
	startCommand = func(cmd *exec.Cmd) error {
		flags = cmd.SysProcAttr.CreationFlags
		return orig(cmd)
	}

	if err := DefaultCommandRunner(context.Background(), t.TempDir(), "exit 0", io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if flags&windows.CREATE_NO_WINDOW == 0 {
		t.Errorf("CreationFlags %#x lack CREATE_NO_WINDOW", flags)
	}
}

func TestExtendedPath(t *testing.T) {
	long := `C:\` + strings.Repeat("a", 300)
	cases := map[string]string{
		`C:\short\f.txt`:             `C:\short\f.txt`,
		long:                         `\\?\` + long,
		`C:\dir\f.`:                  `\\?\C:\dir\f.`,
		`C:\dir\f `:                  `\\?\C:\dir\f `,
		`\\server\share\` + long[3:]: `\\?\UNC\server\share\` + long[3:],
		`\\?\` + long:                `\\?\` + long,
		`\\.\` + long:                `\\.\` + long,
		// The limit counts UTF-16 units, not bytes.
		`C:\` + strings.Repeat("é", 256): `C:\` + strings.Repeat("é", 256),
		`C:\` + strings.Repeat("é", 257): `\\?\C:\` + strings.Repeat("é", 257),
	}
	for in, want := range cases {
		if got := extendedPath(in); got != want {
			t.Errorf("extendedPath(%.20q...) = %.30q..., want %.30q...", in, got, want)
		}
	}
}

func TestDeleteFileTrailingPeriod(t *testing.T) {
	// File.Delete removes a file whose name ends in a period (it uses the
	// \\?\ prefix); without it Windows trims the period and finds no file.
	dir := t.TempDir()
	p := filepath.Join(dir, "f.")
	name, err := windows.UTF16PtrFromString(`\\?\` + p)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	windows.CloseHandle(h)

	if err := deleteFile(p); err != nil {
		t.Fatal(err)
	}
	if _, err := windows.GetFileAttributes(name); err == nil {
		t.Error("file still exists")
	}
}

func TestDeleteFileLongPath(t *testing.T) {
	dir := t.TempDir()
	p := dir
	for len(p) < 300 {
		p = filepath.Join(p, strings.Repeat("d", 50))
	}
	p = filepath.Join(p, "f.txt")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := deleteFile(p); err != nil {
		t.Fatal(err)
	}
	if exists(p) {
		t.Error("file still exists")
	}
}
