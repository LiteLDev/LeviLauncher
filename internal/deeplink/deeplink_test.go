package deeplink

import (
	"strings"
	"testing"
)

func TestParseAccepts(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want Request
	}{
		{"latest", "levilauncher://install/github.com/LiteLDev/Arke",
			Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke"}},
		{"version", "levilauncher://install/github.com/LiteLDev/Arke?version=0.1.6",
			Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke", Version: "0.1.6"}},
		{"version and variant", "levilauncher://install/github.com/LiteLDev/LegacyScriptEngine?version=0.17.0-rc.1&variant=lua",
			Request{Action: "install", ModulePath: "github.com/LiteLDev/LegacyScriptEngine", Version: "0.17.0-rc.1", Variant: "lua"}},
		{"nested variant", "levilauncher://install/github.com/a/b?variant=client/lua",
			Request{Action: "install", ModulePath: "github.com/a/b", Variant: "client/lua"}},
		{"build metadata", "levilauncher://install/github.com/a/b?version=1.0.0%2Bbuild.5",
			Request{Action: "install", ModulePath: "github.com/a/b", Version: "1.0.0+build.5"}},
		{"trailing slash", "levilauncher://install/github.com/LiteLDev/Arke/?version=0.1.6",
			Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke", Version: "0.1.6"}},
		{"percent-encoded path", "levilauncher://install/github.com%2FLiteLDev%2FArke",
			Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke"}},
		{"percent-encoded letters", "levilauncher://install/github.com/LiteLDev/%41rke",
			Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke"}},
		{"case of scheme and action", "LeviLauncher://INSTALL/github.com/LiteLDev/Arke",
			Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke"}},
		{"surrounding whitespace", "  levilauncher://install/github.com/a/b \n",
			Request{Action: "install", ModulePath: "github.com/a/b"}},
		{"unknown params ignored", "levilauncher://install/github.com/a/b?utm_source=site&version=1.2.3&x=",
			Request{Action: "install", ModulePath: "github.com/a/b", Version: "1.2.3"}},
		{"empty version and variant", "levilauncher://install/github.com/a/b?version=&variant=",
			Request{Action: "install", ModulePath: "github.com/a/b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
			// The canonical form, which crosses the single-instance pipe,
			// reads back to the same request.
			again, err := Parse(got.String())
			if err != nil || again != got {
				t.Fatalf("Parse(%q) = %+v, %v; want %+v", got.String(), again, err, got)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"other scheme", "lip://install/github.com/a/b"},
		{"http", "https://levimc.org/mods/github.com/a/b"},
		{"scheme only", "levilauncher:"},
		{"opaque", "levilauncher:install/github.com/a/b"},
		{"no action", "levilauncher:///github.com/a/b"},
		{"unknown action", "levilauncher://uninstall/github.com/a/b"},
		{"run action", "levilauncher://run/github.com/a/b"},
		{"no mod", "levilauncher://install/"},
		{"no mod no slash", "levilauncher://install"},
		{"user info", "levilauncher://evil@install/github.com/a/b"},
		{"port", "levilauncher://install:80/github.com/a/b"},
		{"fragment", "levilauncher://install/github.com/a/b#client"},
		{"empty fragment", "levilauncher://install/github.com/a/b#"},
		{"no dot in host element", "levilauncher://install/LiteLDev/Arke"},
		{"dot dot", "levilauncher://install/github.com/../../Windows/System32"},
		{"encoded dot dot", "levilauncher://install/github.com/a/%2E%2E/b"},
		{"dot element", "levilauncher://install/github.com/./b"},
		{"backslash", `levilauncher://install/github.com/a\..\b`},
		{"encoded backslash", "levilauncher://install/github.com/a%5C..%5Cb"},
		{"windows path", "levilauncher://install/C:/Windows/System32"},
		{"unc path", "levilauncher://install//server/share/x"},
		{"double slash", "levilauncher://install/github.com//b"},
		{"two trailing slashes", "levilauncher://install/github.com/a/b//"},
		{"file url", "levilauncher://install/file:///C:/x"},
		{"url as path", "levilauncher://install/https://github.com/a/b"},
		{"encoded newline in path", "levilauncher://install/github.com/a/b%0Afocus%091"},
		{"raw newline", "levilauncher://install/github.com/a/b\nlaunch\tx"},
		{"raw tab", "levilauncher://install/github.com/a/b\tx"},
		{"space", "levilauncher://install/github.com/a/b c"},
		{"encoded nul", "levilauncher://install/github.com/a/b%00"},
		{"non-ascii", "levilauncher://install/github.com/a/b\u00e9"},
		{"uppercase host element", "levilauncher://install/GitHub.com/a/b"},
		{"leading dash", "levilauncher://install/-github.com/a/b"},
		{"bad escape", "levilauncher://install/github.com/a/b%zz"},
		{"v prefix", "levilauncher://install/github.com/a/b?version=v1.2.3"},
		{"partial version", "levilauncher://install/github.com/a/b?version=1.2"},
		{"leading zero", "levilauncher://install/github.com/a/b?version=01.2.3"},
		{"branch", "levilauncher://install/github.com/a/b?version=main"},
		{"commit", "levilauncher://install/github.com/a/b?version=0123abcd"},
		{"range", "levilauncher://install/github.com/a/b?version=%3E%3D1.0.0"},
		{"version url", "levilauncher://install/github.com/a/b?version=https://evil.example/x.zip"},
		{"version path", "levilauncher://install/github.com/a/b?version=..%2F..%2Fx"},
		{"version twice", "levilauncher://install/github.com/a/b?version=1.0.0&version=2.0.0"},
		{"variant uppercase", "levilauncher://install/github.com/a/b?variant=Client"},
		{"variant path", "levilauncher://install/github.com/a/b?variant=../x"},
		{"variant three levels", "levilauncher://install/github.com/a/b?variant=a/b/c"},
		{"variant twice", "levilauncher://install/github.com/a/b?variant=a&variant=b"},
		{"bad query", "levilauncher://install/github.com/a/b?version=%zz"},
		{"too long", "levilauncher://install/github.com/a/" + strings.Repeat("b", MaxLength)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := Parse(tc.raw); err == nil {
				t.Fatalf("Parse(%q) = %+v, want an error", tc.raw, got)
			}
		})
	}
}

func TestFromArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
		ok   bool
	}{
		{nil, "", false},
		{[]string{"--debug"}, "", false},
		{[]string{"--launch=x"}, "", false},
		{[]string{"levilauncher"}, "", false},
		{[]string{"levilauncher://install/github.com/a/b"}, "levilauncher://install/github.com/a/b", true},
		{[]string{"--debug", "LEVILAUNCHER://install/github.com/a/b"}, "LEVILAUNCHER://install/github.com/a/b", true},
		// Recognised as a link (so it never reaches the command line) even
		// when it is not a valid one; Parse rejects it.
		{[]string{"levilauncher:garbage"}, "levilauncher:garbage", true},
	} {
		got, ok := FromArgs(tc.args)
		if got != tc.want || ok != tc.ok {
			t.Errorf("FromArgs(%q) = %q, %v; want %q, %v", tc.args, got, ok, tc.want, tc.ok)
		}
	}
}

func TestInboxKeepsLatestUntilTaken(t *testing.T) {
	var b Inbox
	if b.Take() != nil {
		t.Fatal("empty inbox returned a link")
	}
	var notified []Request
	b.SetNotify(func(r Request) { notified = append(notified, r) })
	if _, err := b.DeliverRaw("levilauncher://run/x"); err == nil {
		t.Fatal("invalid link delivered")
	}
	first, err := b.DeliverRaw("levilauncher://install/github.com/a/b")
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.DeliverRaw("levilauncher://install/github.com/c/d?version=1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(notified) != 2 || notified[0] != first || notified[1] != second {
		t.Fatalf("notified %+v", notified)
	}
	if got := b.Take(); got == nil || *got != second {
		t.Fatalf("Take() = %+v, want %+v", got, second)
	}
	if got := b.Take(); got != nil {
		t.Fatalf("second Take() = %+v, want nil", got)
	}
}
