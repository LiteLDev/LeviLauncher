package mcservice

import (
	_ "image/gif"
	_ "image/jpeg"

	"os"

	"path/filepath"

	"strings"

	"github.com/liteldev/LeviLauncher/internal/utils"

	"golang.org/x/sys/windows"
)

// userDesktopDir is the signed-in user's desktop (FOLDERID_Desktop), which
// follows a redirection to OneDrive or elsewhere.
func userDesktopDir() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0); err == nil && utils.DirExists(p) {
		return p
	}
	if home := strings.TrimSpace(os.Getenv("USERPROFILE")); home != "" {
		if p := filepath.Join(home, "Desktop"); utils.DirExists(p) {
			return p
		}
	}
	return ""
}
