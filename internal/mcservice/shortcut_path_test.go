package mcservice

import "testing"

func TestShortcutFileNameMatchesCreateDesktopShortcut(t *testing.T) {
	for name, want := range map[string]string{
		"1.21.50":      "Minecraft - 1.21.50.lnk",
		"  Survival  ": "Minecraft - Survival.lnk",
		`a:b/c*d`:      "Minecraft - a_b_c_d.lnk",
		"trail. ":      "Minecraft - trail.lnk",
		"...":          "Minecraft - Minecraft.lnk",
		"tab\there":    "Minecraft - tab_here.lnk",
	} {
		if got := ShortcutFileName(name); got != want {
			t.Errorf("ShortcutFileName(%q) = %q, want %q", name, got, want)
		}
	}
}
