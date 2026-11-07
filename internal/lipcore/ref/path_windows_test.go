package ref

import "testing"

func TestCombinePathWindows(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{`C:\ws`, `x.zip`, `C:\ws\x.zip`},
		{`C:\ws\`, `x.zip`, `C:\ws\x.zip`},
		{`C:\ws`, `D:\y`, `D:\y`},
		{`C:\ws`, `\y`, `\y`},
		{`C:\ws`, `D:y`, `D:y`},
		{`C:`, `x`, `C:\x`},
		{`C:\ws`, ``, `C:\ws`},
	} {
		if got := combinePath(tc.a, tc.b); got != tc.want {
			t.Errorf("combinePath(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestTrimEndingDirectorySeparatorWindows(t *testing.T) {
	for in, want := range map[string]string{
		`C:\`:                `C:\`,
		`C:\a\`:              `C:\a`,
		`\\srv\share\`:       `\\srv\share`,
		`\\srv\share\a\`:     `\\srv\share\a`,
		`\\?\C:\`:            `\\?\C:\`,
		`C:\a`:               `C:\a`,
		`\\srv\`:             `\\srv\`,
		`\\?\UNC\srv\share\`: `\\?\UNC\srv\share`,
	} {
		if got := trimEndingDirectorySeparator(in); got != want {
			t.Errorf("trimEndingDirectorySeparator(%q) = %q, want %q", in, got, want)
		}
	}
}
