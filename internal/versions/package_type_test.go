package versions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMetaDetectsPackageTypeWithoutConfusingGDKRegistration(t *testing.T) {
	for _, tc := range []struct {
		name, explicit, want string
		appx, gdk            bool
	}{
		{"legacy gdk", "", PackageTypeGDK, false, false},
		{"imported uwp", "", PackageTypeUWP, true, false},
		{"registered gdk", "", PackageTypeGDK, true, true},
		{"explicit uwp", PackageTypeUWP, PackageTypeUWP, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteMeta(dir, VersionMeta{Name: "instance", PackageType: tc.explicit}); err != nil {
				t.Fatal(err)
			}
			for name, create := range map[string]bool{"AppxManifest.xml": tc.appx, "MicrosoftGame.config": tc.gdk} {
				if create {
					if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			m, err := ReadMeta(dir)
			if err != nil || m.PackageType != tc.want {
				t.Fatalf("want %s got %+v err=%v", tc.want, m, err)
			}
		})
	}
}
