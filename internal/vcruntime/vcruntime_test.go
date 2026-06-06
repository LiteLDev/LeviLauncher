package vcruntime

import (
	"path/filepath"
	"testing"
)

func TestIsVcRuntimeRegistryStateInstalled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		state vcRuntimeRegistryState
		want  bool
	}{
		{
			name: "installed with version",
			state: vcRuntimeRegistryState{
				Installed:    1,
				HasInstalled: true,
				Version:      "v14.44.35208.0",
			},
			want: true,
		},
		{
			name: "installed with major fallback",
			state: vcRuntimeRegistryState{
				Installed:    1,
				HasInstalled: true,
				Major:        14,
				HasMajor:     true,
			},
			want: true,
		},
		{
			name: "missing installed flag",
			state: vcRuntimeRegistryState{
				Version: "v14.44.35208.0",
			},
			want: false,
		},
		{
			name: "installed flag zero",
			state: vcRuntimeRegistryState{
				Installed:    0,
				HasInstalled: true,
				Version:      "v14.44.35208.0",
			},
			want: false,
		},
		{
			name: "installed but version empty and major too low",
			state: vcRuntimeRegistryState{
				Installed:    1,
				HasInstalled: true,
				Major:        13,
				HasMajor:     true,
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isVcRuntimeRegistryStateInstalled(tc.state)
			if got != tc.want {
				t.Fatalf("isVcRuntimeRegistryStateInstalled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasInstalledVcRuntimeRegistry(t *testing.T) {
	t.Parallel()

	paths := []string{"native", "fallback"}
	states := map[string]vcRuntimeRegistryState{
		"fallback": {
			Installed:    1,
			HasInstalled: true,
			Version:      "v14.44.35208.0",
		},
	}

	got := hasInstalledVcRuntimeRegistry(paths, func(path string) (vcRuntimeRegistryState, bool) {
		state, ok := states[path]
		return state, ok
	})
	if !got {
		t.Fatal("expected installed registry state to be detected")
	}
}

func TestIsVC2015To2022X64RedistributableDisplayName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		displayName string
		want        bool
	}{
		{
			name:        "canonical x64 redistributable",
			displayName: "Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.44.35208.0",
			want:        true,
		},
		{
			name:        "case insensitive x64",
			displayName: "microsoft visual c++ 2015-2022 redistributable x64",
			want:        true,
		},
		{
			name:        "x86 redistributable ignored",
			displayName: "Microsoft Visual C++ 2015-2022 Redistributable (x86) - 14.44.35208.0",
			want:        false,
		},
		{
			name:        "old runtime ignored",
			displayName: "Microsoft Visual C++ 2013 Redistributable (x64) - 12.0.40664",
			want:        false,
		},
		{
			name:        "runtime package ignored",
			displayName: "Microsoft Visual C++ 2022 X64 Minimum Runtime - 14.44.35208",
			want:        false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isVC2015To2022X64RedistributableDisplayName(tc.displayName)
			if got != tc.want {
				t.Fatalf("isVC2015To2022X64RedistributableDisplayName() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasVC2015To2022X64UninstallEntry(t *testing.T) {
	t.Parallel()

	gotRootPath := ""
	got := hasVC2015To2022X64UninstallEntry(func(rootPath string) []string {
		gotRootPath = rootPath
		return []string{
			"Microsoft Visual C++ 2013 Redistributable (x64) - 12.0.40664",
			"Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.44.35208.0",
		}
	})
	if !got {
		t.Fatal("expected x64 2015-2022 redistributable uninstall entry to be detected")
	}
	if gotRootPath != vcRuntimeUninstallRootPath {
		t.Fatalf("root path = %q, want %q", gotRootPath, vcRuntimeUninstallRootPath)
	}

	if hasVC2015To2022X64UninstallEntry(func(string) []string {
		return []string{"Microsoft Visual C++ 2015-2022 Redistributable (x86) - 14.44.35208.0"}
	}) {
		t.Fatal("did not expect x86 uninstall entry to count as installed")
	}
}

func TestHasVC2015To2022X64SystemFiles(t *testing.T) {
	t.Parallel()

	system32Dir := filepath.Join("C:", "Windows", "System32")
	files := make(map[string]bool, len(vcRuntimeX64SystemDLLs))
	for _, dll := range vcRuntimeX64SystemDLLs {
		files[filepath.Join(system32Dir, dll)] = true
	}

	if !hasVC2015To2022X64SystemFiles(system32Dir, func(path string) bool { return files[path] }) {
		t.Fatal("expected complete x64 system DLL set to count as installed")
	}

	delete(files, filepath.Join(system32Dir, "vcruntime140_1.dll"))
	if hasVC2015To2022X64SystemFiles(system32Dir, func(path string) bool { return files[path] }) {
		t.Fatal("did not expect missing vcruntime140_1.dll to count as installed")
	}

	if hasVC2015To2022X64SystemFiles("", func(string) bool { return true }) {
		t.Fatal("did not expect empty system32 path to count as installed")
	}
}

func TestIsVC2015To2022X64Installed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		forceNoInstalled  bool
		hasRegistry       bool
		hasUninstallEntry bool
		hasSystemFiles    bool
		want              bool
	}{
		{
			name:              "force no installed overrides all evidence",
			forceNoInstalled:  true,
			hasRegistry:       true,
			hasUninstallEntry: true,
			hasSystemFiles:    true,
			want:              false,
		},
		{
			name:        "registry installed",
			hasRegistry: true,
			want:        true,
		},
		{
			name:              "uninstall entry installed",
			hasUninstallEntry: true,
			want:              true,
		},
		{
			name:           "x64 system files installed",
			hasSystemFiles: true,
			want:           true,
		},
		{
			name: "no evidence missing",
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isVC2015To2022X64Installed(tc.forceNoInstalled, tc.hasRegistry, tc.hasUninstallEntry, tc.hasSystemFiles)
			if got != tc.want {
				t.Fatalf("isVC2015To2022X64Installed() = %v, want %v", got, tc.want)
			}
		})
	}
}
