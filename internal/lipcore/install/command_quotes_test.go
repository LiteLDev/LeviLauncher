package install

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ports of the tests CommandRunnerTests.cs gained with passing hook commands
// to cmd.exe verbatim (lip 67ba662).

func runQuoted(t *testing.T, dir, command string) {
	t.Helper()
	var out strings.Builder
	if err := DefaultCommandRunner(context.Background(), dir, command, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

func TestRunQuotedPathsReachShellAsWritten(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a b.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := `cp "a b.txt" "c d.txt"`
	if runtime.GOOS == "windows" {
		command = `copy "a b.txt" "c d.txt"`
	}

	runQuoted(t, dir, command)

	if b, err := os.ReadFile(filepath.Join(dir, "c d.txt")); err != nil || string(b) != "content" {
		t.Fatalf("%q %v", b, err)
	}
}

func TestRunQuotedArgumentKeepsQuotes(t *testing.T) {
	dir := t.TempDir()
	command := `echo '"hello"' > out.txt`
	if runtime.GOOS == "windows" {
		command = `echo "hello"> out.txt`
	}

	runQuoted(t, dir, command)

	if b, err := os.ReadFile(filepath.Join(dir, "out.txt")); err != nil || strings.TrimSpace(string(b)) != `"hello"` {
		t.Fatalf("%q %v", b, err)
	}
}

func TestRunCommandStartingAndEndingWithQuotesKeepsQuotes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a b.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	program := "cat"
	if runtime.GOOS == "windows" {
		program = filepath.Join(os.Getenv("SystemRoot"), "System32", "findstr.exe") + `" "content`
	}

	runQuoted(t, dir, `"`+program+`" "a b.txt"> "c d.txt"`)

	if b, err := os.ReadFile(filepath.Join(dir, "c d.txt")); err != nil || strings.TrimSpace(string(b)) != "content" {
		t.Fatalf("%q %v", b, err)
	}
}
