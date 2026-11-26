package tray

import "testing"

func TestSetLabelsBeforeSetupDoesNotPanic(t *testing.T) {
	mu.Lock()
	prevApp, prevHandle := app, handle
	prevShow, prevExit := showLabel, exitLabel
	app, handle = nil, nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		app, handle = prevApp, prevHandle
		showLabel, exitLabel = prevShow, prevExit
		mu.Unlock()
	})

	SetLabels("Show", "Quit")
	if showLabel != "Show" || exitLabel != "Quit" {
		t.Fatalf("labels not updated: show=%q exit=%q", showLabel, exitLabel)
	}
}
