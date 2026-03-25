package lip

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testTarEntry struct {
	name string
	body string
	mode int64
}

func buildTestTarGz(t *testing.T, entries []testTarEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, entry := range entries {
		header := &tar.Header{
			Name: entry.name,
			Mode: entry.mode,
			Size: int64(len(entry.body)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header failed: %v", err)
		}
		if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
			t.Fatalf("write tar body failed: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer failed: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer failed: %v", err)
	}

	return buffer.Bytes()
}

func containsEnvValue(env []string, key string, value string) bool {
	want := key + "=" + value
	for _, entry := range env {
		if strings.EqualFold(strings.TrimSpace(entry), want) {
			return true
		}
	}
	return false
}

func TestResolveLatestTarball(t *testing.T) {
	tests := []struct {
		name            string
		source          lipPackageSource
		expectedTarball string
	}{
		{
			name: "tencent mirror source",
			source: lipPackageSource{
				MetadataURL:       lipMirrorMetadataAPI,
				TarballHost:       lipMirrorTarballHost,
				TarballPathPrefix: lipMirrorTarballPathPrefix,
			},
			expectedTarball: "https://mirrors.cloud.tencent.com/npm/@futrime/lip/-/lip-0.34.4.tgz",
		},
		{
			name: "official npm source",
			source: lipPackageSource{
				MetadataURL:       lipOfficialMetadataAPI,
				TarballHost:       lipOfficialTarballHost,
				TarballPathPrefix: lipOfficialTarballPathPrefix,
			},
			expectedTarball: "https://registry.npmjs.org/@futrime/lip/-/lip-0.34.4.tgz",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version, tarballURL, err := resolveLatestTarball(lipPackageMetadata{
				DistTags: map[string]string{
					"latest": "0.34.4",
					"beta":   "0.34.4-beta.2",
				},
				Versions: map[string]lipPackageVersion{
					"0.34.4": {
						Dist: lipPackageDist{
							Tarball: test.expectedTarball,
						},
					},
					"0.34.4-beta.2": {
						Dist: lipPackageDist{
							Tarball: test.expectedTarball,
						},
					},
				},
			}, test.source)
			if err != nil {
				t.Fatalf("resolveLatestTarball failed: %v", err)
			}
			if version != "0.34.4" {
				t.Fatalf("unexpected version: %q", version)
			}
			if tarballURL != test.expectedTarball {
				t.Fatalf("unexpected tarball URL: %q", tarballURL)
			}
		})
	}
}

func TestResolveLatestTarballErrors(t *testing.T) {
	tests := []struct {
		name      string
		source    lipPackageSource
		meta      lipPackageMetadata
		wantError string
	}{
		{
			name: "missing latest dist tag",
			source: lipPackageSource{
				MetadataURL:       lipOfficialMetadataAPI,
				TarballHost:       lipOfficialTarballHost,
				TarballPathPrefix: lipOfficialTarballPathPrefix,
			},
			meta: lipPackageMetadata{
				DistTags: map[string]string{},
				Versions: map[string]lipPackageVersion{},
			},
			wantError: "ERR_LIP_FETCH_RELEASE",
		},
		{
			name: "missing latest version metadata",
			source: lipPackageSource{
				MetadataURL:       lipOfficialMetadataAPI,
				TarballHost:       lipOfficialTarballHost,
				TarballPathPrefix: lipOfficialTarballPathPrefix,
			},
			meta: lipPackageMetadata{
				DistTags: map[string]string{"latest": "0.34.4"},
				Versions: map[string]lipPackageVersion{},
			},
			wantError: "ERR_LIP_RELEASE_ASSET_NOT_FOUND",
		},
		{
			name: "missing tarball",
			source: lipPackageSource{
				MetadataURL:       lipOfficialMetadataAPI,
				TarballHost:       lipOfficialTarballHost,
				TarballPathPrefix: lipOfficialTarballPathPrefix,
			},
			meta: lipPackageMetadata{
				DistTags: map[string]string{"latest": "0.34.4"},
				Versions: map[string]lipPackageVersion{
					"0.34.4": {},
				},
			},
			wantError: "ERR_LIP_RELEASE_ASSET_NOT_FOUND",
		},
		{
			name: "tarball does not match selected source",
			source: lipPackageSource{
				MetadataURL:       lipMirrorMetadataAPI,
				TarballHost:       lipMirrorTarballHost,
				TarballPathPrefix: lipMirrorTarballPathPrefix,
			},
			meta: lipPackageMetadata{
				DistTags: map[string]string{"latest": "0.34.4"},
				Versions: map[string]lipPackageVersion{
					"0.34.4": {
						Dist: lipPackageDist{
							Tarball: "https://registry.npmjs.org/@futrime/lip/-/lip-0.34.4.tgz",
						},
					},
				},
			},
			wantError: "ERR_LIP_RELEASE_ASSET_NOT_FOUND",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := resolveLatestTarball(test.meta, test.source)
			if err == nil {
				t.Fatal("expected resolveLatestTarball to fail")
			}
			if got := extractErrCode(err); got != test.wantError {
				t.Fatalf("unexpected error code: got %q want %q (%v)", got, test.wantError, err)
			}
		})
	}
}

func TestHasLipDotNetMajorInListRuntimesOutput(t *testing.T) {
	output := strings.Join([]string{
		"Microsoft.AspNetCore.App 10.0.0 [C:\\Program Files\\dotnet\\shared\\Microsoft.AspNetCore.App]",
		"Microsoft.NETCore.App 10.0.1 [C:\\Program Files\\dotnet\\shared\\Microsoft.NETCore.App]",
		"",
	}, "\n")
	if !hasLipDotNetMajorInListRuntimesOutput(output) {
		t.Fatal("expected .NET 10 runtime to be detected from dotnet --list-runtimes output")
	}

	if hasLipDotNetMajorInListRuntimesOutput("Microsoft.NETCore.App 9.0.9 [C:\\Program Files\\dotnet\\shared\\Microsoft.NETCore.App]") {
		t.Fatal("did not expect .NET 9 runtime to satisfy the .NET 10 requirement")
	}
}

func TestLipCommandEnvAddsDotNetRootForLocalInstall(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "shared", lipDotNetRuntimeName, "10.0.1")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatalf("create runtime dir failed: %v", err)
	}

	t.Setenv("DOTNET_ROOT", root)
	env := lipCommandEnv()
	if !containsEnvValue(env, "DOTNET_ROOT", root) {
		t.Fatalf("expected DOTNET_ROOT=%q in command env, got %v", root, env)
	}
}

func TestExtractLipBinaryFromTarGz(t *testing.T) {
	archive := buildTestTarGz(t, []testTarEntry{
		{name: "package/win32-arm64/lipd.exe", body: "wrong-arm64", mode: 0o755},
		{name: "package/win32-x64/lip.exe", body: "wrong-cli", mode: 0o755},
		{name: "package/win32-x64/lipd.exe", body: "expected-daemon", mode: 0o755},
	})

	dest := filepath.Join(t.TempDir(), lipBinaryName)
	if err := extractLipBinaryFromTarGz(bytes.NewReader(archive), dest); err != nil {
		t.Fatalf("extractLipBinaryFromTarGz failed: %v", err)
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted binary failed: %v", err)
	}
	if string(content) != "expected-daemon" {
		t.Fatalf("unexpected extracted content: %q", string(content))
	}
}

func TestExtractLipBinaryFromTarGzMissingTarget(t *testing.T) {
	archive := buildTestTarGz(t, []testTarEntry{
		{name: "package/win32-arm64/lipd.exe", body: "wrong-arm64", mode: 0o755},
		{name: "package/win32-x64/lip.exe", body: "wrong-cli", mode: 0o755},
	})

	dest := filepath.Join(t.TempDir(), lipBinaryName)
	err := extractLipBinaryFromTarGz(bytes.NewReader(archive), dest)
	if err == nil {
		t.Fatal("expected extractLipBinaryFromTarGz to fail")
	}
	if got := extractErrCode(err); got != "ERR_LIP_EXTRACT_BINARY" {
		t.Fatalf("unexpected error code: got %q want %q (%v)", got, "ERR_LIP_EXTRACT_BINARY", err)
	}
}
