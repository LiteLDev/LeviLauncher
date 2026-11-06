//go:build windows

package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	// File.Delete makes the path full first, which trims the trailing
	// period: deleting `f.` removes `f`, not a file named `f.`.
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
	if err := os.WriteFile(filepath.Join(dir, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := deleteFile(p); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(dir, "f")) {
		t.Error("f still exists")
	}
	if _, err := windows.GetFileAttributes(name); err != nil {
		t.Errorf("f. was deleted: %v", err)
	}
	// Now `f` is missing, which is not an error.
	if err := deleteFile(p); err != nil {
		t.Fatal(err)
	}
}

func writeScript(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultCommandRunnerWaitsForBackgroundProcess(t *testing.T) {
	// A process the script starts in the background holds the output pipes
	// after the script exits; like CliWrap the runner waits for it and
	// succeeds with its output.
	dir := t.TempDir()
	writeScript(t, dir, "bg.cmd", "@ping -n 4 127.0.0.1 >nul\r\n@echo background done\r\n")
	var out strings.Builder
	start := time.Now()
	err := DefaultCommandRunner(context.Background(), dir, `start /b bg.cmd`, &out, io.Discard)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(start); elapsed < 2500*time.Millisecond {
		t.Errorf("returned after %v, before the background process ended", elapsed)
	}
	if !strings.Contains(out.String(), "background done") {
		t.Errorf("output %q", out.String())
	}
}

func TestDefaultCommandRunnerCancelAfterScriptExit(t *testing.T) {
	// Cancelling while only a background process still holds the pipes
	// kills it and returns promptly.
	dir := t.TempDir()
	writeScript(t, dir, "bg.cmd", "@ping -n 6 127.0.0.1 >nul\r\n@echo late> late.txt\r\n")
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := DefaultCommandRunner(ctx, dir, `start /b bg.cmd`, io.Discard, io.Discard)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("returned after %v", elapsed)
	}
	time.Sleep(6*time.Second - elapsed)
	if exists(filepath.Join(dir, "late.txt")) {
		t.Error("background process survived cancellation")
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
