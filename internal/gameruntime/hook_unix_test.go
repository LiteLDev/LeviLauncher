//go:build darwin

package gameruntime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeHookWine(t *testing.T, body string) Profile {
	t.Helper()
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix")
	if err := os.MkdirAll(filepath.Join(prefix, "drive_c"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(prefix, "dosdevices"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/", filepath.Join(prefix, "dosdevices", "z:")); err != nil {
		t.Fatal(err)
	}
	wine := filepath.Join(dir, "wine")
	if err := os.WriteFile(wine, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return Profile{Wine: wine, Prefix: prefix}
}

func TestRunHookToleratesInheritedOutputHandlesOnZeroExit(t *testing.T) {
	// Simulates Wine services (wineserver, services.exe) keeping redirected
	// stdout/stderr pipe handles open after cmd.exe exits 0.
	p := writeFakeHookWine(t, `
echo "PeEditor finished"
(sleep 10) &
exit 0
`)
	var out bytes.Buffer
	if err := p.RunHook(context.Background(), t.TempDir(), "echo ok", &out); err != nil {
		t.Fatalf("RunHook should succeed when child exits 0 with lingering pipe holders, got: %v", err)
	}
	if !strings.Contains(out.String(), "PeEditor finished") {
		t.Fatalf("expected captured hook output, got %q", out.String())
	}
}

func TestRunHookFailsOnNonZeroExitWithInheritedHandles(t *testing.T) {
	p := writeFakeHookWine(t, `
echo "PeEditor failed"
(sleep 10) &
exit 7
`)
	var out bytes.Buffer
	if err := p.RunHook(context.Background(), t.TempDir(), "echo fail", &out); err == nil {
		t.Fatal("RunHook must fail when child exits non-zero")
	}
}

func TestRunHookFailsOnCancelledContext(t *testing.T) {
	p := writeFakeHookWine(t, `
sleep 10
exit 0
`)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	if err := p.RunHook(ctx, t.TempDir(), "echo slow", &out); err == nil {
		t.Fatal("RunHook must fail when context is cancelled")
	}
}
