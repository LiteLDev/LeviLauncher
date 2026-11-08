package mcservice

import (
	"testing"
	"time"
)

func TestSunTimesAtShanghaiEquinox(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	got := sunTimesAt(time.Date(2026, 3, 20, 10, 0, 0, 0, loc), 31.2)
	// Real Shanghai values are ~05:58 / 18:08; the lon-from-offset estimate
	// (120°E vs 121.5°E) must stay within a few minutes.
	check := func(label, s, want string) {
		a, _ := time.Parse("15:04", s)
		b, _ := time.Parse("15:04", want)
		if d := a.Sub(b); d < -10*time.Minute || d > 10*time.Minute {
			t.Errorf("%s = %s, want about %s", label, s, want)
		}
	}
	check("sunrise", got.Sunrise, "05:58")
	check("sunset", got.Sunset, "18:08")
}

func TestSunTimesAtPolarFallsBack(t *testing.T) {
	got := sunTimesAt(time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 80)
	if got.Sunrise != "06:00" || got.Sunset != "18:00" {
		t.Errorf("polar day fallback = %+v", got)
	}
}

func TestSunTimesAtIgnoresDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable:", err)
	}
	// New York, 2026-06-21: real sunrise ~05:25 EDT, sunset ~20:31 EDT. The
	// estimate uses 75°W (vs 74°W) and must not be shifted by the DST hour.
	got := sunTimesAt(time.Date(2026, 6, 21, 12, 0, 0, 0, loc), 40.7)
	check := func(label, s, want string) {
		a, _ := time.Parse("15:04", s)
		b, _ := time.Parse("15:04", want)
		if d := a.Sub(b); d < -10*time.Minute || d > 10*time.Minute {
			t.Errorf("%s = %s, want about %s", label, s, want)
		}
	}
	check("sunrise", got.Sunrise, "05:25")
	check("sunset", got.Sunset, "20:31")
}
