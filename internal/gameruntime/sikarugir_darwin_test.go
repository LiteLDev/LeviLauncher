package gameruntime

import (
	"strings"
	"testing"
)

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
