//go:build !windows

package apppath

import (
	"os"
	"strings"
)

func roamingAppData() string {
	if v := strings.TrimSpace(os.Getenv("APPDATA")); v != "" {
		return v
	}
	v, _ := os.UserConfigDir()
	return v
}

func localAppData() string {
	if v := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); v != "" {
		return v
	}
	v, _ := os.UserCacheDir()
	return v
}
