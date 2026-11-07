package lipindex

import (
	"reflect"
	"sort"
	"testing"
)

func TestCompareSemverLike(t *testing.T) {
	list := []string{"1.0.0-rc.1", "v1.0.0", "0.9.10", "0.9.2", "nightly", "1.0.0-beta", "1.0.0-rc.10", "1.0.0-rc.2", "abc"}
	sort.SliceStable(list, func(i, j int) bool { return CompareSemverLike(list[i], list[j]) > 0 })
	want := []string{"v1.0.0", "1.0.0-rc.10", "1.0.0-rc.2", "1.0.0-rc.1", "1.0.0-beta", "0.9.10", "0.9.2", "nightly", "abc"}
	if !reflect.DeepEqual(list, want) {
		t.Fatalf("sorted = %v, want %v", list, want)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.21.50.07", "1.21.50.7", 0},
		{"1.21.100", "1.21.99", 1},
		{"1.21", "1.21.0", 0},
		{"1.26.0", "1.26.0.1", -1},
		{"0.9.2", "0.9.10", -1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); sign(got) != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}

func TestInRange(t *testing.T) {
	cases := []struct {
		version, expr string
		want          bool
	}{
		{"1.4.2", ">=1.4.0 <1.5.0", true},
		{"1.5.0", ">=1.4.0 <1.5.0", false},
		{"1.5.0-rc.1", "<1.5.0", true},
		{"26.60.3", "26.60.*", true},
		{"26.61.0", "26.60.*", false},
		{"2.0.0", "1.x", false},
		{"1.9.9", "1.x", true},
		{"9.9.9", "*", true},
		{"1.2.3", "1.2.3", true},
		{"v1.2.3", "=1.2.3", true},
		{"1.2.4", "1.2.3", false},
		{"1.2.3", "", false},
		{"bad", "*", false},
		{"1.2.3", ">= 1.2.0", false}, // tokens split on spaces, as in the window
	}
	for _, c := range cases {
		if got := InRange(c.version, c.expr); got != c.want {
			t.Errorf("InRange(%q, %q) = %v, want %v", c.version, c.expr, got, c.want)
		}
	}
	if !Compatible("1.4.2", []string{"2.x", "1.4.*"}) || Compatible("1.4.2", nil) {
		t.Error("Compatible should match any range and nothing without ranges")
	}
}

func TestLoaderDB(t *testing.T) {
	db := NormalizeLoaderDB(map[string][]string{
		"v1.21.50.07": {"1.0.0", " 1.1.0", "1.1.0", "1.2.0-rc.1"},
		"1.21.60.10":  {"1.3.0"},
		"1.21.70.01":  {},
	})
	if got := db.Versions("1.21.50.07"); !reflect.DeepEqual(got, []string{"1.2.0-rc.1", "1.1.0", "1.0.0"}) {
		t.Fatalf("For = %v", got)
	}
	if _, ok := db["1.21.70.01"]; ok {
		t.Fatal("a game without versions must be dropped")
	}
	games := SupportedGameVersions([]string{">=1.1.0"}, db)
	if !reflect.DeepEqual(games, []string{"1.21.60.10", "1.21.50.07"}) {
		t.Fatalf("SupportedGameVersions = %v", games)
	}
	if len(SupportedGameVersions(nil, db)) != 0 {
		t.Fatal("no ranges supports nothing")
	}
}
