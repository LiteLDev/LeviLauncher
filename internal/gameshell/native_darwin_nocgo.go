//go:build darwin && !cgo

package gameshell

import "errors"

func showNativeDialogPlatform(DialogRequest) (DialogResult, error) {
	return DialogResult{}, errors.New("native dialog requires cgo")
}

func focusNativeDialogPlatform() {}
