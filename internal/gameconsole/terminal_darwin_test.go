//go:build darwin

package gameconsole

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSelectOpenArgs(t *testing.T) {
	script := "/tmp/levi-console/console.command"
	cases := []struct {
		name        string
		cmdHandler  string
		unixHandler string
		want        []string
	}{
		{
			name:        "Ghostty default for both",
			cmdHandler:  "com.mitchellh.ghostty",
			unixHandler: "com.mitchellh.ghostty",
			want:        []string{script},
		},
		{
			name:        "Stock macOS Terminal",
			cmdHandler:  "com.apple.Terminal",
			unixHandler: "com.apple.Terminal",
			want:        []string{script},
		},
		{
			name:        "iTerm2 default for .command",
			cmdHandler:  "com.googlecode.iterm2",
			unixHandler: "com.apple.Terminal",
			want:        []string{script},
		},
		{
			name:        "Custom terminal matching unix-executable handler",
			cmdHandler:  "com.example.MyCustomTerm",
			unixHandler: "com.example.mycustomterm",
			want:        []string{script},
		},
		{
			name:        ".command hijacked by VS Code while Ghostty handles unix-executable",
			cmdHandler:  "com.microsoft.VSCode",
			unixHandler: "com.mitchellh.ghostty",
			want:        []string{"-b", "com.mitchellh.ghostty", script},
		},
		{
			name:        ".command hijacked by editor with no known unix terminal",
			cmdHandler:  "com.microsoft.VSCode",
			unixHandler: "com.apple.TextEdit",
			want:        []string{"-b", "com.apple.Terminal", script},
		},
		{
			name:        "No handler resolved falls back to default open",
			cmdHandler:  "",
			unixHandler: "",
			want:        []string{script},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectOpenArgs(script, tc.cmdHandler, tc.unixHandler)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("selectOpenArgs(%q, %q) = %v, want %v", tc.cmdHandler, tc.unixHandler, got, tc.want)
			}
		})
	}
}

func TestDefaultHandlerProbe(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "probe.command")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write probe script: %v", err)
	}
	cmdHandler := defaultBundleIDForFile(script)
	unixHandler := defaultBundleIDForUTI("public.unix-executable")
	args := selectOpenArgs(script, cmdHandler, unixHandler)
	t.Logf("detected handlers: .command=%q public.unix-executable=%q -> open %v", cmdHandler, unixHandler, args)
	if len(args) == 0 {
		t.Fatal("selectOpenArgs returned empty args")
	}
}
