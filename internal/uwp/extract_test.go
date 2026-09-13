package uwp

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testManifest(arch string) string {
	return `<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10"><Identity Name="Microsoft.MinecraftUWP" Publisher="CN=Microsoft Corporation, O=Microsoft Corporation, L=Redmond, S=Washington, C=US" Version="1.21.9301.0" ProcessorArchitecture="` + arch + `"/><Applications><Application Id="App" Executable="Minecraft.Windows.exe"/></Applications></Package>`
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, contents := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func writeArchive(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.appx")
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInstallAppxAndPreserveExistingTarget(t *testing.T) {
	archive := writeArchive(t, zipBytes(t, map[string]string{"AppxManifest.xml": testManifest("neutral"), "Minecraft.Windows.exe": "test-executable", "data/test.txt": "content"}))
	target := filepath.Join(t.TempDir(), "instance")
	m, err := Install(context.Background(), archive, target, Options{Prepare: func(dir string, m Manifest) error {
		return os.WriteFile(filepath.Join(dir, "version.json"), []byte(m.GameVersion()), 0644)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if m.GameVersion() != "1.21.93.1" {
		t.Fatalf("game version: %s", m.GameVersion())
	}
	if b, err := os.ReadFile(filepath.Join(target, "data", "test.txt")); err != nil || string(b) != "content" {
		t.Fatalf("extracted content: %q, %v", b, err)
	}
	if _, err := Install(context.Background(), archive, target, Options{}); ErrorCode(err) != "ERR_TARGET_EXISTS" {
		t.Fatalf("overwrite was not rejected: %v", err)
	}
}

func TestInstallRejectsUnsafeAndInvalidPackagesAtomically(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		code  string
	}{
		{"traversal", map[string]string{"AppxManifest.xml": testManifest("neutral"), "Minecraft.Windows.exe": "x", "../escape": "x"}, "ERR_UWP_UNSAFE_ARCHIVE"},
		{"alternate stream", map[string]string{"AppxManifest.xml": testManifest("neutral"), "Minecraft.Windows.exe": "x", "foo:bar": "x"}, "ERR_UWP_UNSAFE_ARCHIVE"},
		{"reserved path", map[string]string{"AppxManifest.xml": testManifest("neutral"), "Minecraft.Windows.exe": "x", "con.txt": "x"}, "ERR_UWP_UNSAFE_ARCHIVE"},
		{"case collision", map[string]string{"AppxManifest.xml": testManifest("neutral"), "Minecraft.Windows.exe": "x", "DATA": "a", "data": "b"}, "ERR_UWP_UNSAFE_ARCHIVE"},
		{"wrong identity", map[string]string{"AppxManifest.xml": strings.Replace(testManifest("neutral"), ReleasePackageName, "Other.Application", 1), "Minecraft.Windows.exe": "x"}, "ERR_UWP_NOT_MINECRAFT"},
		{"missing executable", map[string]string{"AppxManifest.xml": testManifest("neutral")}, "ERR_NOT_FOUND_EXE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			archive := writeArchive(t, zipBytes(t, tc.files))
			target := filepath.Join(t.TempDir(), "instance")
			if _, err := Install(context.Background(), archive, target, Options{}); ErrorCode(err) != tc.code {
				t.Fatalf("want %s got %v", tc.code, err)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("failed install published target: %v", err)
			}
		})
	}
}

func TestBundleSelectsApplicationArchitecture(t *testing.T) {
	manifest := `<Bundle><Packages><Package Type="application" Architecture="x86" FileName="x86.appx"/><Package Type="application" Architecture="x64" FileName="x64.appx"/></Packages></Bundle>`
	data := zipBytes(t, map[string]string{"AppxMetadata/AppxBundleManifest.xml": manifest, "x86.appx": "x86", "x64.appx": "x64"})
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ arch, want string }{{"amd64", "x64.appx"}, {"386", "x86.appx"}, {"arm64", "x64.appx"}} {
		f, err := selectBundlePackage(r, tc.arch)
		if err != nil || f.Name != tc.want {
			t.Fatalf("%s selected %v: %v", tc.arch, f, err)
		}
	}
}

func TestInstallRejectsResourceBundleWithoutPublishingInstance(t *testing.T) {
	inner := zipBytes(t, map[string]string{"AppxManifest.xml": testManifest("neutral"), "Minecraft.Windows.exe": "x"})
	for _, resourceType := range []string{"resource", "Resource"} {
		t.Run(resourceType, func(t *testing.T) {
			// Put the resource after the matching application so that selection
			// cannot silently succeed before examining every bundle entry.
			manifest := `<Bundle><Packages><Package Type="application" Architecture="neutral" FileName="game.appx"/><Package Type="` + resourceType + `" Architecture="neutral" FileName="resources.appx"/></Packages></Bundle>`
			archive := writeArchive(t, zipBytes(t, map[string]string{"AppxMetadata/AppxBundleManifest.xml": manifest, "game.appx": string(inner), "resources.appx": "resource"}))
			parent := t.TempDir()
			target := filepath.Join(parent, "instance")
			prepared := false
			_, err := Install(context.Background(), archive, target, Options{Prepare: func(stage string, _ Manifest) error {
				prepared = true
				return os.WriteFile(filepath.Join(stage, "version.json"), []byte(`{"packageType":"uwp"}`), 0644)
			}})
			if ErrorCode(err) != "ERR_UWP_RESOURCE_BUNDLE" {
				t.Fatalf("resource bundle was not rejected: %v", err)
			}
			if prepared {
				t.Fatal("unsupported bundle reached metadata preparation")
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("unsupported bundle published an instance: %v", err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatalf("unsupported bundle left installation residue: %v, %v", entries, err)
			}
		})
	}
}

func TestInstallBundleAndCancellation(t *testing.T) {
	inner := zipBytes(t, map[string]string{"AppxManifest.xml": testManifest("neutral"), "Minecraft.Windows.exe": "x"})
	data := zipBytes(t, map[string]string{"AppxMetadata/AppxBundleManifest.xml": `<Bundle><Packages><Package Type="application" Architecture="neutral" FileName="game.appx"/></Packages></Bundle>`, "game.appx": string(inner)})
	archive := writeArchive(t, data)
	if _, err := Install(context.Background(), archive, filepath.Join(t.TempDir(), "instance"), Options{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Install(ctx, archive, filepath.Join(t.TempDir(), "cancelled"), Options{}); ErrorCode(err) != "ERR_CANCELED" {
		t.Fatalf("cancellation: %v", err)
	}
}
