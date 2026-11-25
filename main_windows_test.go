package main

import (
	"os"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/deeplink"
)

func TestUpdateStartupWaitsForPreviousInstance(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	for _, tc := range []struct {
		name string
		args []string
		url  string
		wait bool
	}{
		{"normal launch", nil, "/", false},
		{"elevated update", []string{"--self-update=1.0.2"}, "/#/updating", true},
		{"post update restart", []string{"--post-update-restart"}, "/", true},
		{"update then restart flag", []string{"--self-update=1.0.2", "--post-update-restart"}, "/#/updating", true},
		{"restart then update flag", []string{"--post-update-restart", "--self-update=1.0.2"}, "/#/updating", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Args = append([]string{"LeviLauncher.exe"}, tc.args...)
			url, _, wait, _ := parseArgs()
			if url != tc.url || wait != tc.wait {
				t.Fatalf("parseArgs() = (url=%q, wait=%v), want (%q, %v)", url, wait, tc.url, tc.wait)
			}
		})
	}
}

func TestDeepLinkFromArgs(t *testing.T) {
	link, err := deepLinkFromArgs([]string{"--debug", "levilauncher://install/github.com/LiteLDev/Arke?version=0.1.6"})
	if err != nil || link == nil || link.ModulePath != "github.com/LiteLDev/Arke" || link.Version != "0.1.6" {
		t.Fatalf("deepLinkFromArgs = %+v, %v", link, err)
	}
	if link, err := deepLinkFromArgs([]string{"--launch=x"}); link != nil || err != nil {
		t.Fatalf("no link: deepLinkFromArgs = %+v, %v", link, err)
	}
	if link, err := deepLinkFromArgs([]string{"levilauncher://install/C:/Windows"}); link != nil || err == nil {
		t.Fatalf("invalid link: deepLinkFromArgs = %+v, %v", link, err)
	}
}

func TestSingleInstanceCommand(t *testing.T) {
	link := &deeplink.Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke", Version: "0.1.6"}
	for _, tc := range []struct {
		version string
		link    *deeplink.Request
		want    string
	}{
		{"", nil, "focus\t1\n"},
		{" My Instance ", nil, "launch\tMy Instance\n"},
		{"", link, "uri\tlevilauncher://install/github.com/LiteLDev/Arke?version=0.1.6\n"},
		{"My Instance", link, "uri\tlevilauncher://install/github.com/LiteLDev/Arke?version=0.1.6\n"},
	} {
		if got := singleInstanceCommand(tc.version, tc.link); got != tc.want {
			t.Errorf("singleInstanceCommand(%q, %v) = %q, want %q", tc.version, tc.link, got, tc.want)
		}
	}
}

func TestSingleInstanceActions(t *testing.T) {
	var focused int
	var launched []string
	links := &deeplink.Inbox{}
	a := singleInstanceActions{
		focus:  func() { focused++ },
		launch: func(name string) { launched = append(launched, name) },
		links:  links,
	}
	for _, line := range []string{
		"focus\t1",
		"launch\tMy Instance",
		"launch\t" + `..\..\evil`,
		"launch\t",
		"uri\tlevilauncher://run/github.com/a/b",
		"uri\tlevilauncher://install/github.com/a/../b",
		"uri\thttps://example.com/x",
		"uri",
		"unknown\tx",
		"",
	} {
		a.handle(line)
	}
	if focused != 1 || len(launched) != 1 || launched[0] != "My Instance" {
		t.Fatalf("focused %d, launched %q", focused, launched)
	}
	if r := links.Take(); r != nil {
		t.Fatalf("an invalid link was delivered: %+v", r)
	}
	a.handle(singleInstanceCommand("", &deeplink.Request{Action: "install", ModulePath: "github.com/LiteLDev/Arke", Variant: "client"}))
	if r := links.Take(); r == nil || r.ModulePath != "github.com/LiteLDev/Arke" || r.Variant != "client" {
		t.Fatalf("link not delivered: %+v", r)
	}
}
