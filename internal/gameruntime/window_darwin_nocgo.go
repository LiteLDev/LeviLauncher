//go:build darwin && !cgo

package gameruntime

func WindowVisible(int) bool { return false }
