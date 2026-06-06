package webview2runtime

import (
	"testing"

	winreg "golang.org/x/sys/windows/registry"
)

func TestIsWebView2RegistryVersionInstalled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "installed version",
			version: "124.0.2478.80",
			want:    true,
		},
		{
			name:    "trimmed installed version",
			version: " 124.0.2478.80 ",
			want:    true,
		},
		{
			name:    "empty version",
			version: "",
			want:    false,
		},
		{
			name:    "placeholder version",
			version: "0.0.0.0",
			want:    false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isWebView2RegistryVersionInstalled(tc.version)
			if got != tc.want {
				t.Fatalf("isWebView2RegistryVersionInstalled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasInstalledWebView2Registry(t *testing.T) {
	t.Parallel()

	entries := []webView2RegistryEntry{
		{root: winreg.LOCAL_MACHINE, path: "machine", accessModes: []uint32{winreg.READ}},
		{root: winreg.CURRENT_USER, path: "user", accessModes: []uint32{winreg.READ}},
	}
	versions := map[string]string{
		"machine": "",
		"user":    "124.0.2478.80",
	}

	got := hasInstalledWebView2Registry(entries, func(_ winreg.Key, path string, _ []uint32) (string, bool) {
		version, ok := versions[path]
		return version, ok
	})
	if !got {
		t.Fatal("expected installed WebView2 registry version to be detected")
	}

	if hasInstalledWebView2Registry(entries, func(_ winreg.Key, path string, _ []uint32) (string, bool) {
		return "0.0.0.0", true
	}) {
		t.Fatal("did not expect placeholder registry version to count as installed")
	}
}
