//go:build darwin || linux

package uwp

import (
	"context"
	"errors"
)

func Register(context.Context, string) error {
	return failure("ERR_UWP_UNSUPPORTED_ON_HOST", errors.ErrUnsupported)
}
func Unregister(context.Context, string) error {
	return failure("ERR_UWP_UNSUPPORTED_ON_HOST", errors.ErrUnsupported)
}
func Launch(context.Context, string) (int, error) {
	return 0, failure("ERR_UWP_UNSUPPORTED_ON_HOST", errors.ErrUnsupported)
}
func LaunchWithPreparation(context.Context, string, bool, func() error, func()) (int, error) {
	return 0, failure("ERR_UWP_UNSUPPORTED_ON_HOST", errors.ErrUnsupported)
}
