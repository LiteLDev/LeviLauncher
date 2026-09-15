package uwp

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func legacyAuthManifest() Manifest {
	return Manifest{
		Identity:     Identity{Version: "1.19.6301.0"},
		Applications: []Application{{ID: "App", Executable: "Minecraft.Windows.exe"}},
	}
}

func TestAuthKeyPreparationUsesContentsInsteadOfVersion(t *testing.T) {
	for _, tc := range []struct {
		version, contents, marker string
	}{
		{"0.1600.5.0", legacyAuthPublicKey, "patched\n"},
		{"1.19.6301.0", legacyAuthPublicKey, "patched\n"},
		{"1.19.7002.0", legacyAuthPublicKey, "patched\n"},
		{"1.19.8002.0", legacyAuthPublicKey, "patched\n"},
		{"1.19.8101.0", legacyAuthPublicKey, "patched\n"},
		{"1.20.1.0", legacyAuthPublicKey + "\x00" + currentAuthPublicKey, "already-current\n"},
		{"1.20.1201.0", legacyAuthPublicKey + "\x00" + currentAuthPublicKey, "already-current\n"},
		{"1.21.11401.0", currentAuthPublicKey, "already-current\n"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			m := legacyAuthManifest()
			m.Identity.Version = tc.version
			dir := t.TempDir()
			exe := filepath.Join(dir, m.Applications[0].Executable)
			if err := os.WriteFile(exe, []byte(tc.contents), 0644); err != nil {
				t.Fatal(err)
			}
			if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
				t.Fatal(err)
			}
			assertAuthMarker(t, dir, tc.marker)
			want := tc.contents
			if tc.marker == "patched\n" {
				want = currentAuthPublicKey
			}
			got, err := os.ReadFile(exe)
			if err != nil || string(got) != want {
				t.Fatalf("unexpected executable contents: %v", err)
			}
		})
	}
}

func TestAuthPublicKeysAreValidAndEqualLength(t *testing.T) {
	if len(legacyAuthPublicKey) != len(currentAuthPublicKey) || legacyAuthPublicKey == currentAuthPublicKey {
		t.Fatal("keys must differ while preserving length")
	}
	for _, key := range []string{legacyAuthPublicKey, currentAuthPublicKey} {
		der, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := x509.ParsePKIXPublicKey(der); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyAuthKeyExactReplacementAndMarker(t *testing.T) {
	dir := t.TempDir()
	m := legacyAuthManifest()
	exe := filepath.Join(dir, m.Applications[0].Executable)
	data := bytes.Repeat([]byte{0xCC}, 3*authKeyScanSize)
	for _, offset := range []int{0, authKeyScanSize - len(legacyAuthPublicKey)/2, 2 * authKeyScanSize, len(data) - len(legacyAuthPublicKey)} {
		copy(data[offset:], legacyAuthPublicKey)
	}
	// Preserve near matches elsewhere in the file.
	copy(data[1000:], legacyAuthPublicKey[:len(legacyAuthPublicKey)-1]+"!")
	if err := os.WriteFile(exe, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
		t.Fatal(err)
	}
	want := bytes.ReplaceAll(data, []byte(legacyAuthPublicKey), []byte(currentAuthPublicKey))
	got, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("executable differs outside exact key replacements: %v", err)
	}
	assertAuthMarker(t, dir, "patched\n")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("expected only executable and marker, without backup: %v, %v", entries, err)
	}
	// No executable exists now: success proves the marker bypasses file I/O,
	// including scanning, hashing or checking whether the patch is still present.
	if err := os.Remove(exe); err != nil {
		t.Fatal(err)
	}
	if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
		t.Fatalf("completed migration opened executable again: %v", err)
	}
}

func TestLegacyAuthKeyNoOpResults(t *testing.T) {
	for _, tc := range []struct{ name, data, state string }{
		{"already patched", "prefix" + currentAuthPublicKey + "suffix", "already-current\n"},
		{"both keys old first", legacyAuthPublicKey + "\x00" + currentAuthPublicKey, "already-current\n"},
		{"both keys new first", currentAuthPublicKey + "\x00" + legacyAuthPublicKey, "already-current\n"},
		{"new key in later chunk", legacyAuthPublicKey + strings.Repeat("\x00", authKeyScanSize) + currentAuthPublicKey, "already-current\n"},
		{"no known key", "historical executable without the known key", "not-applicable\n"},
		{"partial key", legacyAuthPublicKey[:len(legacyAuthPublicKey)-1], "not-applicable\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			m := legacyAuthManifest()
			exe := filepath.Join(dir, m.Applications[0].Executable)
			if err := os.WriteFile(exe, []byte(tc.data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(exe)
			if err != nil || string(got) != tc.data {
				t.Fatalf("no-op modified executable: %v", err)
			}
			assertAuthMarker(t, dir, tc.state)
			if err := os.Remove(exe); err != nil {
				t.Fatal(err)
			}
			if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
				t.Fatalf("no-op completion scanned executable again: %v", err)
			}
		})
	}
}

func TestLegacyAuthKeyFailuresRemainRetryable(t *testing.T) {
	dir := t.TempDir()
	m := legacyAuthManifest()
	exe := filepath.Join(dir, m.Applications[0].Executable)
	if err := ensureLegacyAuthKey(context.Background(), dir, m); !os.IsNotExist(errors.Unwrap(err)) {
		t.Fatalf("missing executable: %v", err)
	}
	assertAuthMarker(t, dir, "")
	if err := os.WriteFile(exe, []byte(legacyAuthPublicKey), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ensureLegacyAuthKey(ctx, dir, m); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	assertAuthMarker(t, dir, "")
	// A truncated marker is not accepted as completion.
	if err := os.WriteFile(filepath.Join(dir, authKeyMarkerName), []byte("pat"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureLegacyAuthKey(context.Background(), dir, m); err != nil {
		t.Fatal(err)
	}
	assertAuthMarker(t, dir, "patched\n")
}

type authKeyReadFailure struct{}

func (authKeyReadFailure) ReadAt([]byte, int64) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestLegacyAuthKeyScanErrors(t *testing.T) {
	if _, _, err := scanLegacyAuthKey(context.Background(), authKeyReadFailure{}); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("read error was swallowed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := scanLegacyAuthKey(ctx, bytes.NewReader(nil)); !errors.Is(err, context.Canceled) {
		t.Fatalf("scan ignored cancellation: %v", err)
	}
}

func TestInstallPatchesLegacyAuthKeyBeforePublishing(t *testing.T) {
	for _, exe := range []string{"Minecraft.Windows.exe", "Minecraft.Win10.DX11.exe"} {
		t.Run(exe, func(t *testing.T) {
			manifest := strings.Replace(testManifest("neutral"), "1.21.9301.0", "1.16.4002.0", 1)
			manifest = strings.Replace(manifest, "Minecraft.Windows.exe", exe, 1)
			archive := writeArchive(t, zipBytes(t, map[string]string{
				"AppxManifest.xml": manifest,
				exe:                "prefix" + legacyAuthPublicKey + "suffix",
				authKeyMarkerName:  "patched\n",
			}))
			target := filepath.Join(t.TempDir(), "instance")
			_, err := Install(context.Background(), archive, target, Options{Prepare: func(stage string, _ Manifest) error {
				data, err := os.ReadFile(filepath.Join(stage, exe))
				if err != nil || string(data) != "prefix"+currentAuthPublicKey+"suffix" {
					t.Fatalf("installation did not patch in staging: %v", err)
				}
				assertAuthMarker(t, stage, "patched\n")
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			assertAuthMarker(t, target, "patched\n")
			// Renames carry the marker; settings use a separate metadata file.
			renamed := target + "-renamed"
			if err := os.Rename(target, renamed); err != nil {
				t.Fatal(err)
			}
			assertAuthMarker(t, renamed, "patched\n")
		})
	}
}

func TestInstallPreservesBothAuthKeys(t *testing.T) {
	contents := legacyAuthPublicKey + "\x00" + currentAuthPublicKey
	archive := writeArchive(t, zipBytes(t, map[string]string{
		"AppxManifest.xml":      strings.Replace(testManifest("neutral"), "1.21.9301.0", "1.20.1.0", 1),
		"Minecraft.Windows.exe": contents,
	}))
	dir := filepath.Join(t.TempDir(), "modern")
	if _, err := Install(context.Background(), archive, dir, Options{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Minecraft.Windows.exe"))
	if err != nil || string(data) != contents {
		t.Fatalf("modern package was modified: %v", err)
	}
	assertAuthMarker(t, dir, "already-current\n")
}

func assertAuthMarker(t *testing.T, dir, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, authKeyMarkerName))
	if want == "" {
		if !os.IsNotExist(err) {
			t.Fatalf("unexpected completion marker: %q, %v", data, err)
		}
		return
	}
	if err != nil || string(data) != want {
		t.Fatalf("marker = %q, %v; want %q", data, err, want)
	}
}
