package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicLeavesNoStagingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.json")
	for _, content := range []string{"one", "two"} {
		if err := WriteFileAtomic(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(path); string(got) != content {
			t.Fatalf("content = %q, want %q", got, content)
		}
	}
	ents, _ := os.ReadDir(filepath.Dir(path))
	if len(ents) != 1 {
		t.Fatalf("staging files left: %v", ents)
	}
	if !IsTempName(filepath.Base(PartialPath(path))) {
		t.Fatal("PartialPath is not a staging name")
	}
}
