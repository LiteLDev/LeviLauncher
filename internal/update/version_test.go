package update

import (
	"os"
	"testing"
)

func TestSetAppVersion(t *testing.T) {
	oldVersion, oldBeta := appVersion, isBeta
	t.Cleanup(func() { appVersion, isBeta = oldVersion, oldBeta })

	cases := []struct {
		stamped string
		want    string
		beta    bool
	}{
		{"", devVersion, false},
		{"  ", devVersion, false},
		{devVersion, devVersion, false},
		{"1.2.3", "1.2.3", false},
		{"v1.2.3", "1.2.3", false},
		{" 1.2.3\n", "1.2.3", false},
		{"1.2.3-beta.1", "1.2.3-beta.1", true},
		{"v2.0.0-rc.1", "2.0.0-rc.1", true},
		{"1.2.3+build.5", "1.2.3+build.5", false},
		{"1.2", invalidStampVersion, false},
		{"feature-x", invalidStampVersion, false},
	}
	for _, c := range cases {
		t.Run(c.stamped, func(t *testing.T) {
			setAppVersion(c.stamped)
			if got := GetAppVersion(); got != c.want {
				t.Errorf("GetAppVersion() = %q, want %q", got, c.want)
			}
			if got := IsBeta(); got != c.beta {
				t.Errorf("IsBeta() = %v, want %v", got, c.beta)
			}
		})
	}
}

// TestInjectedVersion checks the version resolved at package init. Without
// stamping it expects the dev default; to verify -ldflags stamping run e.g.
//
//	LEVILAUNCHER_EXPECT_VERSION=1.2.3-beta.1 LEVILAUNCHER_EXPECT_BETA=true go test \
//	  -ldflags "-X github.com/liteldev/LeviLauncher/internal/update.appVersion=1.2.3-beta.1" \
//	  ./internal/update -run TestInjectedVersion
func TestInjectedVersion(t *testing.T) {
	want, wantBeta := devVersion, false
	if v := os.Getenv("LEVILAUNCHER_EXPECT_VERSION"); v != "" {
		want, wantBeta = v, os.Getenv("LEVILAUNCHER_EXPECT_BETA") == "true"
	}
	if got := GetAppVersion(); got != want {
		t.Fatalf("GetAppVersion() = %q, want %q", got, want)
	}
	if got := IsBeta(); got != wantBeta {
		t.Fatalf("IsBeta() = %v, want %v", got, wantBeta)
	}
}

func TestIsDevVersion(t *testing.T) {
	for v, want := range map[string]bool{
		devVersion:              true,
		"v" + devVersion:        true,
		" " + devVersion + "\n": true,
		"0.0.0":                 false,
		"0.0.0-ci":              false,
		"1.2.3":                 false,
		"1.2.3-beta.1":          false,
	} {
		if got := isDevVersion(v); got != want {
			t.Errorf("isDevVersion(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestSelectUpdateReleaseSkipsDevVersion(t *testing.T) {
	releases := []githubRelease{{TagName: "v99.0.0"}}
	for _, v := range []string{devVersion, "v" + devVersion} {
		if rel, ok := selectUpdateRelease(releases, v); ok {
			t.Errorf("selectUpdateRelease(%q) offered %q, want no update", v, rel.TagName)
		}
	}
}
