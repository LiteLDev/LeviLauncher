//go:build linux

package gameinput

import (
    "context"
    "sync"
    "github.com/wailsapp/wails/v3/pkg/application"
)

var (
    mu       sync.Mutex
    ensuring bool
)

func IsInstalled() bool { return true }

func EnsureInteractive(ctx context.Context) {
    mu.Lock()
    if ensuring { mu.Unlock(); return }
    ensuring = true
    mu.Unlock()
    defer func(){ mu.Lock(); ensuring = false; mu.Unlock() }()
    application.Get().Event.Emit("gameinput.ensure.start", struct{}{})
    application.Get().Event.Emit("gameinput.ensure.done", struct{}{})
}