//go:build darwin && !cgo

package gameruntime

func WindowVisible(int) bool { return false }

func mainDisplayRetinaOversampleRatio() float64 { return 1.0 }
