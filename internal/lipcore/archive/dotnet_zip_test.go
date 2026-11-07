package archive

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// testdata/dotnet_zip_entries.json was produced by a C# harness over lip
// v0.38.0-beta.1's Lip.Core: for each ZIP file it lists ZipSource.Keys and,
// for each key, what ZipSource.OpenRead serves (base64) or the message of
// the exception it throws. The archives exercise what .NET 10's
// System.IO.Compression does and does not check: bad and zero CRC-32s,
// encryption flags, uncompressed sizes that do not match the data,
// truncated, corrupt and over-long Deflate data, an unsupported method and
// compressed sizes past the end of the file. Real lip.exe 0.38.0-beta.1
// installs files with a bad CRC-32 or the encryption flag as they are
// stored.
func TestZipSource_MatchesDotnet(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "dotnet_zip_entries.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Zip     string
		Entries []struct {
			Key     string
			Content *string
			Err     string
		}
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range cases {
		data, err := base64.StdEncoding.DecodeString(c.Zip)
		if err != nil {
			t.Fatal(err)
		}
		s, err := OpenZipBytes(data)
		if err != nil {
			t.Fatalf("OpenZipBytes: %v", err)
		}
		var keys []string
		for _, e := range c.Entries {
			keys = append(keys, e.Key)
		}
		if got := s.Keys(); !slices.Equal(got, keys) {
			t.Errorf("keys %q, want %q", got, keys)
		}
		for _, e := range c.Entries {
			got, err := readAll(s, e.Key)
			if e.Err != "" {
				var de *DamagedError
				if !errors.As(err, &de) || err.Error() != e.Err {
					t.Errorf("%s: got %q, %v; want error %q", e.Key, got, err, e.Err)
				}
				continue
			}
			want, _ := base64.StdEncoding.DecodeString(*e.Content)
			if err != nil || string(got) != string(want) {
				t.Errorf("%s: got %d bytes %q..., %v; want %d bytes", e.Key, len(got), got[:min(len(got), 16)], err, len(want))
			}
		}
	}
}

func readAll(s Source, key string) ([]byte, error) {
	rc, err := s.Open(key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
