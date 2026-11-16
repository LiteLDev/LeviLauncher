//go:build darwin && !cgo

package nativeinstall

import "errors"

func systemProtectedKey(string, bool) ([]byte, error) {
	return nil, errors.New("macOS secure storage requires a CGO-enabled native build")
}
