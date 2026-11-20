package gameruntime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyMacInputSettingsSkipsUnchanged(t *testing.T) {
	prefix := t.TempDir()
	// Disabled by default with no marker: zero-overhead no-op (does not invoke Wine).
	p := Profile{Prefix: prefix, LeftCommandIsCtrl: false}
	if err := p.ApplyMacInputSettings(context.Background()); err != nil {
		t.Fatalf("default disabled should be a no-op, got: %v", err)
	}
	// Enabled with matching marker: zero-overhead no-op.
	if err := os.WriteFile(filepath.Join(prefix, ".levilauncher-mac-input"), []byte("1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p.LeftCommandIsCtrl = true
	if err := p.ApplyMacInputSettings(context.Background()); err != nil {
		t.Fatalf("matching marker should be a no-op, got: %v", err)
	}
}

func TestMinecraftWrapperCommands(t *testing.T) {
	generated := minecraftWrapperCommands("")
	if strings.HasSuffix(strings.TrimSpace(generated), ";") {
		t.Fatal("generated commands have a trailing shell separator")
	}
	if got := minecraftWrapperCommands(generated + "; "); got != generated {
		t.Fatalf("legacy configuration was not repaired: %q", got)
	}
	for _, current := range []string{generated, "export CUSTOM_FLAG=1", "export XGR_MODS=1; echo custom"} {
		updated := minecraftWrapperCommands(current)
		if !strings.HasSuffix(updated, current) {
			t.Fatalf("user commands changed: %q -> %q", current, updated)
		}
		if got := minecraftWrapperCommands(updated); got != updated {
			t.Fatalf("commands changed on the second launch: %q -> %q", updated, got)
		}
	}
}
