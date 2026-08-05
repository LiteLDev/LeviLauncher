package webview2runtime

import (
	"os"
	"path/filepath"
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

func TestResolveStartupOptionsUsesConfigAndCLIOverride(t *testing.T) {
	root := t.TempDir()
	configRuntime := createWebView2TestRuntime(t, filepath.Join(root, "config-runtime"))
	cliRuntime := createWebView2TestRuntime(t, filepath.Join(root, "cli-runtime"))
	configPath := filepath.Join(root, StartupConfigFileName)
	config := []byte(`{"browserExecutableFolder":"config-runtime"}`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	fromConfig, err := ResolveStartupOptions(nil, configPath)
	if err != nil {
		t.Fatalf("ResolveStartupOptions(config): %v", err)
	}
	if fromConfig.BrowserExecutableFolder != configRuntime {
		t.Fatalf("config runtime = %q, want %q", fromConfig.BrowserExecutableFolder, configRuntime)
	}
	if fromConfig.Source != StartupRuntimeSourceConfig {
		t.Fatalf("config source = %q, want %q", fromConfig.Source, StartupRuntimeSourceConfig)
	}

	fromCLI, err := ResolveStartupOptions(
		[]string{"--webview2-runtime-dir", cliRuntime},
		configPath,
	)
	if err != nil {
		t.Fatalf("ResolveStartupOptions(cli): %v", err)
	}
	if fromCLI.BrowserExecutableFolder != cliRuntime {
		t.Fatalf("CLI runtime = %q, want %q", fromCLI.BrowserExecutableFolder, cliRuntime)
	}
	if fromCLI.Source != StartupRuntimeSourceCLI {
		t.Fatalf("CLI source = %q, want %q", fromCLI.Source, StartupRuntimeSourceCLI)
	}
}

func TestResolveStartupOptionsSupportsEqualsArgument(t *testing.T) {
	root := t.TempDir()
	runtimeDir := createWebView2TestRuntime(t, filepath.Join(root, "runtime"))
	options, err := ResolveStartupOptions(
		[]string{"--webview2-runtime-dir=" + runtimeDir},
		filepath.Join(root, StartupConfigFileName),
	)
	if err != nil {
		t.Fatalf("ResolveStartupOptions(): %v", err)
	}
	if options.BrowserExecutableFolder != runtimeDir {
		t.Fatalf("runtime = %q, want %q", options.BrowserExecutableFolder, runtimeDir)
	}
}

func TestResolveStartupOptionsRejectsInvalidConfiguration(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, StartupConfigFileName)

	if err := os.WriteFile(configPath, []byte(`{"browserExecutableFolder":`), 0o644); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}
	if _, err := ResolveStartupOptions(nil, configPath); err == nil {
		t.Fatal("expected malformed startup configuration to fail")
	}

	if err := os.WriteFile(configPath, []byte(`{"browserExecutableFolder":"missing"}`), 0o644); err != nil {
		t.Fatalf("write invalid runtime config: %v", err)
	}
	if _, err := ResolveStartupOptions(nil, configPath); err == nil {
		t.Fatal("expected missing runtime directory to fail")
	}
}

func TestResolveStartupOptionsRejectsMissingArgumentValue(t *testing.T) {
	if _, err := ResolveStartupOptions(
		[]string{"--webview2-runtime-dir"},
		filepath.Join(t.TempDir(), StartupConfigFileName),
	); err == nil {
		t.Fatal("expected missing command-line value to fail")
	}
}

func createWebView2TestRuntime(t *testing.T, runtimeDir string) string {
	t.Helper()
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, webView2RuntimeExecutableName), []byte("runtime"), 0o644); err != nil {
		t.Fatalf("write runtime executable: %v", err)
	}
	absolute, err := filepath.Abs(runtimeDir)
	if err != nil {
		t.Fatalf("abs runtime path: %v", err)
	}
	return absolute
}
