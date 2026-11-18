//go:build darwin

package processinfo

import "os"

func IsElevated() bool { return os.Geteuid() == 0 }
