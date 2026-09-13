//go:build windows

package nativeinstall

import "unsafe"

func ptr[T any](v *T) uintptr { return uintptr(unsafe.Pointer(v)) }
