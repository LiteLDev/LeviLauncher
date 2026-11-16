//go:build darwin || linux

package update

import "errors"

func startSelfUpdateElevated(string, string) error {
	return errors.New("native builds use manual application updates; Windows elevation is unavailable")
}
