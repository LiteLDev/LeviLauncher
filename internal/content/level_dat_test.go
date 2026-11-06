package content

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
)

func TestLevelDatRootAndNestedEdits(t *testing.T) {
	dir := t.TempDir()
	root := map[string]any{
		"outside": int16(7),
		"Data": map[string]any{
			"nested": map[string]any{"ints": [2]int32{2, 3}},
			"bytes":  [2]byte{1, 2},
		},
	}
	if err := EncodeLevelDat(dir, 9, root); err != nil {
		t.Fatal(err)
	}
	for _, nestedView := range []bool{false, true} {
		read := ReadLevelDatFields
		if nestedView {
			read = func(dir string) ([]types.LevelDatField, int32, error) { return ReadLevelDatFieldsAt(dir, nil) }
		}
		fields, version, err := read(dir)
		if err != nil || version != 9 || len(fields) != 2 {
			t.Fatalf("read: %v, %d, %v", fields, version, err)
		}
		for _, field := range fields {
			if !field.InData || field.Path == nil ||
				(field.Name == "nested" && (field.ValueJSON == "") != nestedView) ||
				(field.Name == "bytes" && field.ValueJSON != `"AQI="`) {
				t.Fatalf("nested view %v: %+v", nestedView, field)
			}
		}
	}
	if err := WriteLevelDatFields(dir, []types.LevelDatField{{Name: "title", Tag: "string", ValueString: "hello"}}, 9); err != nil {
		t.Fatal(err)
	}
	if err := WriteLevelDatFieldsAt(dir, []string{"nested"}, []types.LevelDatField{{Name: "ints", Tag: "list", ValueJSON: "[4,5,6]"}}, 9); err != nil {
		t.Fatal(err)
	}
	data := root["Data"].(map[string]any)
	data["title"] = "hello"
	data["nested"].(map[string]any)["ints"] = [3]int32{4, 5, 6}
	got, version, err := DecodeLevelDat(dir)
	if err != nil || version != 9 || !reflect.DeepEqual(got, root) {
		t.Fatalf("round trip: %#v, %d, %v", got, version, err)
	}
}

func TestLevelDatWriteDistinguishesMissingAndCorruptFile(t *testing.T) {
	dir := t.TempDir()
	fields := []types.LevelDatField{{Name: "answer", Tag: "int", ValueString: "42"}}
	if err := WriteLevelDatFields(dir, fields, 8); err != nil {
		t.Fatalf("create missing file: %v", err)
	}
	file := filepath.Join(dir, "level.dat")
	broken := []byte("unreadable NBT")
	if err := os.WriteFile(file, broken, 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteLevelDatFieldsAt(dir, []string{"nested"}, fields, 8); err == nil {
		t.Fatal("overwrote a corrupt level.dat")
	}
	if got, err := os.ReadFile(file); err != nil || !bytes.Equal(got, broken) {
		t.Fatalf("original: %q, %v", got, err)
	}
}
