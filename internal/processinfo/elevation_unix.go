//go:build darwin || linux

package processinfo

import "os"

func IsElevated() bool { return os.Geteuid() == 0 }
