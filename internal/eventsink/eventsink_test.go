package eventsink

import (
	"context"
	"testing"
)

func TestRouting(t *testing.T) {
	// Without an application and without a default sink, events vanish.
	Emit("nothing", 1)
	if HasApplication() {
		t.Fatal("tests have no Wails application")
	}

	var global, scoped []string
	SetDefault(Func(func(name string, _ ...any) { global = append(global, name) }))
	t.Cleanup(func() { SetDefault(nil) })

	Emit("a")
	From(context.Background()).Emit("b")
	ctx := With(context.Background(), Func(func(name string, _ ...any) { scoped = append(scoped, name) }))
	From(ctx).Emit("c")
	From(context.WithValue(ctx, struct{}{}, 1)).Emit("d")

	if len(global) != 2 || global[0] != "a" || global[1] != "b" {
		t.Fatalf("default sink got %q", global)
	}
	if len(scoped) != 2 || scoped[0] != "c" || scoped[1] != "d" {
		t.Fatalf("context sink got %q", scoped)
	}
}
