package mcservice

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/uwp"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

func TestInstallExtractAppxWritesManifestChannelAndMetadata(t *testing.T) {
	for _, tc := range []struct{ name, identity, requested, want string }{
		{"release", uwp.ReleasePackageName, "release", "release"},
		{"beta", uwp.ReleasePackageName, "beta", "beta"},
		{"local preview", uwp.PreviewPackageName, "release", "preview"},
		{"wrong preview selection", uwp.ReleasePackageName, "preview", "release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			apppath.SetBaseRootOverride(root)
			t.Cleanup(func() { apppath.SetBaseRootOverride("") })
			var b bytes.Buffer
			zw := zip.NewWriter(&b)
			files := map[string]string{
				"AppxManifest.xml":      `<Package><Identity Name="` + tc.identity + `" Publisher="CN=Microsoft Corporation, O=Microsoft Corporation, L=Redmond, S=Washington, C=US" Version="1.21.9301.0" ProcessorArchitecture="neutral"/><Applications><Application Id="App" Executable="Minecraft.Windows.exe"/></Applications></Package>`,
				"Minecraft.Windows.exe": "not a real executable",
			}
			for name, data := range files {
				f, err := zw.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.Write([]byte(data)); err != nil {
					t.Fatal(err)
				}
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(root, "test.appx")
			if err := os.WriteFile(archive, b.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			if code := InstallExtractAppx(context.Background(), archive, "instance", tc.requested); code != "" {
				t.Fatalf("install error: %s", code)
			}
			dir := filepath.Join(root, "versions", "instance")
			m, err := versions.ReadMeta(dir)
			if err != nil {
				t.Fatal(err)
			}
			if m.Type != tc.want || m.PackageType != versions.PackageTypeUWP || m.GameVersion != "1.21.93.1" || m.Registered || m.EnableIsolation || m.EnableConsole {
				t.Fatalf("unexpected metadata: %+v", m)
			}
			for _, file := range []string{"LeviLauncher.dll", "Minecraft.Windows.original.exe"} {
				if _, err := os.Stat(filepath.Join(dir, file)); !os.IsNotExist(err) {
					t.Fatalf("GDK artifact created: %s", file)
				}
			}
		})
	}
}
