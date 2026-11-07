// Package eventsink decouples backend progress and status events from the
// Wails application, so the same code runs in the window and headless.
//
// Backend code emits through From(ctx) when it has a context, or through
// Emit when it does not. Where the events go:
//
//  1. a sink attached to the context with With (per-operation routing);
//  2. otherwise the process-wide sink installed with SetDefault (the CLI
//     installs one at start-up);
//  3. otherwise the Wails application's event bus, when an application
//     exists (the GUI: behaviour unchanged);
//  4. otherwise nowhere.
package eventsink

import (
	"context"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Sink receives backend events. Implementations must be safe for concurrent
// use: events arrive from many goroutines.
type Sink interface {
	Emit(name string, data ...any)
}

// Func adapts a function to a Sink.
type Func func(name string, data ...any)

// Emit calls f.
func (f Func) Emit(name string, data ...any) {
	if f != nil {
		f(name, data...)
	}
}

// Discard drops every event.
var Discard Sink = Func(func(string, ...any) {})

type ctxKey struct{}

// With returns a context whose events go to s instead of the default sink.
func With(ctx context.Context, s Sink) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, s)
}

// FromContext returns the sink attached to ctx, if any.
func FromContext(ctx context.Context) (Sink, bool) {
	if ctx == nil {
		return nil, false
	}
	s, ok := ctx.Value(ctxKey{}).(Sink)
	return s, ok && s != nil
}

// From returns the sink for events of an operation running under ctx: the
// context's sink, else Default().
func From(ctx context.Context) Sink {
	if s, ok := FromContext(ctx); ok {
		return s
	}
	return Default()
}

type holder struct{ s Sink }

var defaultSink atomic.Pointer[holder]

// SetDefault installs the process-wide sink used when a context carries
// none. nil restores the Wails fallback. The GUI never calls it.
func SetDefault(s Sink) {
	if s == nil {
		defaultSink.Store(nil)
		return
	}
	defaultSink.Store(&holder{s: s})
}

// Default returns the process-wide sink: the one installed with SetDefault,
// else the Wails application's event bus (a no-op without an application).
func Default() Sink {
	if h := defaultSink.Load(); h != nil {
		return h.s
	}
	return wails
}

// Emit sends an event to Default(), for code that has no context.
func Emit(name string, data ...any) {
	Default().Emit(name, data...)
}

// HasApplication reports whether a Wails application exists in this process,
// that is whether this is the GUI. Window behaviours (minimise, restore,
// quit, Discord presence) are applied only then.
func HasApplication() bool {
	return application.Get() != nil
}

var wails Sink = Func(func(name string, data ...any) {
	if app := application.Get(); app != nil && app.Event != nil {
		app.Event.Emit(name, data...)
	}
})
