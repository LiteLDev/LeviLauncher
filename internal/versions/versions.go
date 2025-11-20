package versions

import (
	"regexp"
	"strings"
)

func ValidateFolderName(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "ERR_NAME_REQUIRED"
	}
	if len(n) > 64 {
		return "ERR_NAME_TOO_LONG"
	}
	if strings.HasSuffix(n, ".") || strings.HasSuffix(n, " ") {
		return "ERR_NAME_TRAILING_DOT_SPACE"
	}
	invalidRe := regexp.MustCompile(`[<>:"/\\|?*]`)
	if invalidRe.MatchString(n) {
		return "ERR_NAME_INVALID_CHAR"
	}
	for _, r := range n {
		if r < 32 {
			return "ERR_NAME_CONTROL_CHAR"
		}
	}
	return ""
}
