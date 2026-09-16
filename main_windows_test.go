package main

import (
	"os"
	"testing"
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
