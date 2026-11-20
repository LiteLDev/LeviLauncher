package versionlaunch

import (
	"os"

	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"

	"github.com/liteldev/LeviLauncher/internal/versions"
)

type Launcher struct {
}

func New(enablePreloader bool) *Launcher {
	return &Launcher{}
}

func isPathInside(root string, target string) bool {
	absRoot, err1 := filepath.Abs(strings.TrimSpace(root))
	absTarget, err2 := filepath.Abs(strings.TrimSpace(target))
	if err1 != nil || err2 != nil {
		return false
	}
	lowRoot := strings.ToLower(filepath.Clean(absRoot))
	lowTarget := strings.ToLower(filepath.Clean(absTarget))
	if lowTarget == lowRoot {
		return true
	}
	return strings.HasPrefix(lowTarget, lowRoot+string(os.PathSeparator))
}

func resolveVersionDir(name string) (string, string) {
	vdir, err := apppath.InstancesDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "", "ERR_ACCESS_VERSIONS_DIR"
	}
	n := strings.TrimSpace(name)
	if n == "" {
		return "", "ERR_INVALID_NAME"
	}
	if filepath.IsAbs(n) {
		return "", "ERR_PATH_ESCAPE"
	}
	if strings.ContainsAny(n, `/\`) {
		return "", "ERR_PATH_ESCAPE"
	}
	cleaned := filepath.Clean(n)
	if cleaned == "." || cleaned == ".." {
		return "", "ERR_PATH_ESCAPE"
	}
	if verr := versions.ValidateFolderName(n); verr != "" {
		return "", "ERR_INVALID_NAME"
	}
	dir := filepath.Join(vdir, n)
	if !isPathInside(vdir, dir) {
		return "", "ERR_PATH_ESCAPE"
	}
	return dir, ""
}

func ValidateLaunchName(name string) string {
	_, errCode := resolveVersionDir(name)
	return errCode
}

func parseCommandLineArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuote := false

	for _, r := range input {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t', '\n', '\r':
			if inQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
