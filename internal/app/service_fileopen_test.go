package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOpenedContentFileBatches(t *testing.T) {
	dir := t.TempDir()
	var files []string
	for _, name := range []string{"测试 Pack.MCPACK", "world.mcworld", "mod.mcmod", "addon.mcaddon", "ignored.zip"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		files = append(files, path)
	}

	directory := filepath.Join(dir, "directory.mcpack")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	args := append(files, directory, filepath.Join(dir, "missing.mcmod"))
	if got := ContentFileArgs(args); !reflect.DeepEqual(got, files[:4]) {
		t.Fatalf("accepted files = %q, want %q", got, files[:4])
	}

	t.Cleanup(func() {
		for FileOpens.TakeFiles() != nil {
		}
	})
	QueueContentFiles(args)
	for _, want := range [][]string{files[:2], files[2:3], files[3:4], nil} {
		if got := FileOpens.TakeFiles(); !reflect.DeepEqual(got, want) {
			t.Fatalf("next batch = %q, want %q", got, want)
		}
	}
}
