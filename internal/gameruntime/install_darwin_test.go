package gameruntime

import (
	"testing"

	"github.com/liteldev/LeviLauncher/internal/types"
)

func TestMainlandLocale(t *testing.T) {
	for locale, expected := range map[string]bool{
		"zh_CN": true, "en_CN": true, "zh-Hans-CN": true, "zh_CN@calendar=chinese": true,
		"": false, "zh": false, "zh_HK": false, "zh-Hant-TW": false, "en_US": false,
	} {
		if got := mainlandLocale(locale); got != expected {
			t.Errorf("mainlandLocale(%q) = %t, want %t", locale, got, expected)
		}
	}
}

func TestWineDownloadSettingsRoundTrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	saved := types.WineDownloadSettings{Mirror: false, GithubProxy: "https://gh-proxy.org"}
	if err := SaveDownloadSettings(saved); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDownloadSettings()
	if err != nil || got.Mirror || got.GithubProxy != defaultWineProxy {
		t.Fatalf("unexpected Wine settings: %+v (%v)", got, err)
	}
}
