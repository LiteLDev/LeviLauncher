//go:build darwin

package gameconsole

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var knownTerminalBundleIDs = map[string]struct{}{
	"com.apple.terminal":       {},
	"com.mitchellh.ghostty":    {},
	"com.googlecode.iterm2":    {},
	"dev.warp.warp-stable":     {},
	"dev.warp.warp":            {},
	"com.github.wez.wezterm":   {},
	"net.kovidgoyal.kitty":     {},
	"org.alacritty":            {},
	"io.alacritty":             {},
	"co.zeit.hyper":            {},
	"dev.rio.rio":              {},
	"org.tabby":                {},
	"com.panic.prompt":         {},
	"com.raphaelamorim.rio":    {},
	"com.termius-dmg.mac":      {},
	"com.cmuxterm.app":         {},
}

func isKnownTerminalBundleID(bundleID string) bool {
	id := strings.ToLower(strings.TrimSpace(bundleID))
	if id == "" {
		return false
	}
	_, ok := knownTerminalBundleIDs[id]
	return ok
}

// selectOpenArgs picks `open` arguments so macOS launches the user's default
// terminal (e.g., Ghostty, iTerm2, Warp, Terminal.app) while avoiding text
// editors if `.command` was reassigned to one.
func selectOpenArgs(scriptPath, commandHandlerBundleID, unixExecHandlerBundleID string) []string {
	cmdHandler := strings.TrimSpace(commandHandlerBundleID)
	unixHandler := strings.TrimSpace(unixExecHandlerBundleID)
	if isKnownTerminalBundleID(cmdHandler) || (cmdHandler != "" && strings.EqualFold(cmdHandler, unixHandler)) {
		return []string{scriptPath}
	}
	if isKnownTerminalBundleID(unixHandler) {
		return []string{"-b", unixHandler, scriptPath}
	}
	if cmdHandler == "" {
		return []string{scriptPath}
	}
	return []string{"-b", "com.apple.Terminal", scriptPath}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func OpenDefaultTerminal(instance, addr, token string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		exe = resolved
	}
	tempDir, err := os.MkdirTemp("", "levi-console-")
	if err != nil {
		return err
	}
	if err := os.Chmod(tempDir, 0700); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}
	tokenFile := filepath.Join(tempDir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(token+"\n"), 0600); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}
	scriptPath := filepath.Join(tempDir, "console.command")
	script := fmt.Sprintf("#!/bin/sh\nrm -f \"$0\"\nexec %s --attach-console --addr %s --token-file %s --title %s\n",
		shellQuote(exe),
		shellQuote(addr),
		shellQuote(tokenFile),
		shellQuote(instance),
	)
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}
	cmdHandler := defaultBundleIDForFile(scriptPath)
	unixHandler := defaultBundleIDForUTI("public.unix-executable")
	args := selectOpenArgs(scriptPath, cmdHandler, unixHandler)
	cmd := exec.Command("/usr/bin/open", args...)
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}
	return nil
}
