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
		{"", devVersion, true},
		{"  ", devVersion, true},
		{devVersion, devVersion, true},
		{"1.2.3", "1.2.3", false},
		{"v1.2.3", "1.2.3", false},
		{" 1.2.3\n", "1.2.3", false},
		{"1.2.3-beta.1", "1.2.3-beta.1", true},
		{"v2.0.0-rc.1", "2.0.0-rc.1", true},
		{"1.2.3+build.5", "1.2.3+build.5", false},
		{"1.2", devVersion, true},
		{"feature-x", devVersion, true},
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
	want, wantBeta := devVersion, true
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
