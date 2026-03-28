package vcruntime

import "testing"

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
				Version:      "v14.50.35719.00",
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
				Version: "v14.50.35719.00",
			},
			want: false,
		},
		{
			name: "installed flag zero",
			state: vcRuntimeRegistryState{
				Installed:    0,
				HasInstalled: true,
				Version:      "v14.50.35719.00",
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
		{
			name: "installed but no version or major",
			state: vcRuntimeRegistryState{
				Installed:    1,
				HasInstalled: true,
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

func TestHasInstalledVcRuntime(t *testing.T) {
	t.Parallel()

	makeReader := func(states map[string]vcRuntimeRegistryState) func(string) (vcRuntimeRegistryState, bool) {
		return func(path string) (vcRuntimeRegistryState, bool) {
			state, ok := states[path]
			return state, ok
		}
	}

	cases := []struct {
		name   string
		states map[string]vcRuntimeRegistryState
		want   bool
	}{
		{
			name: "native registry path present",
			states: map[string]vcRuntimeRegistryState{
				vcRuntimeRegistryPaths[0]: {
					Installed:    1,
					HasInstalled: true,
					Version:      "v14.50.35719.00",
				},
			},
			want: true,
		},
		{
			name: "wow6432 registry path present",
			states: map[string]vcRuntimeRegistryState{
				vcRuntimeRegistryPaths[1]: {
					Installed:    1,
					HasInstalled: true,
					Major:        14,
					HasMajor:     true,
				},
			},
			want: true,
		},
		{
			name:   "official keys missing",
			states: map[string]vcRuntimeRegistryState{},
			want:   false,
		},
		{
			name: "official key exists but invalid",
			states: map[string]vcRuntimeRegistryState{
				vcRuntimeRegistryPaths[0]: {
					Installed:    1,
					HasInstalled: true,
					Major:        13,
					HasMajor:     true,
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := hasInstalledVcRuntime(vcRuntimeRegistryPaths, makeReader(tc.states))
			if got != tc.want {
				t.Fatalf("hasInstalledVcRuntime() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasVCUninstallEvidenceFromDisplayNames(t *testing.T) {
	t.Parallel()

	displayNames := []string{
		"Microsoft Visual C++ 2022 X64 Minimum Runtime - 14.50.35719",
		"Microsoft Visual C++ v14 Redistributable (x64) - 14.50.35719",
		"Microsoft Visual C++ 2013 x64 Minimum Runtime - 12.0.40664",
	}

	if !hasVCUninstallEvidenceFromDisplayNames(displayNames) {
		t.Fatal("expected uninstall evidence to be detected")
	}

	if hasVCUninstallEvidenceFromDisplayNames([]string{
		"Microsoft Visual C++ 2013 x64 Minimum Runtime - 12.0.40664",
		"Some Other Package",
	}) {
		t.Fatal("did not expect old runtimes or unrelated packages to count as VC14 runtime evidence")
	}
}

func TestOfficialRegistryKeysRemainTheOnlyInstallSignal(t *testing.T) {
	t.Parallel()

	if hasInstalledVcRuntime(vcRuntimeRegistryPaths, func(string) (vcRuntimeRegistryState, bool) {
		return vcRuntimeRegistryState{}, false
	}) {
		t.Fatal("expected missing official registry keys to be treated as not installed")
	}

	if !hasVCUninstallEvidenceFromDisplayNames([]string{
		"Microsoft Visual C++ 2022 X64 Additional Runtime - 14.50.35719",
	}) {
		t.Fatal("expected uninstall entries to still be detectable for debug logging")
	}
}
