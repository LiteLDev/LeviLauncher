package gameruntime

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func WindowVisible(pid int) bool {
	data, err := exec.Command("xprop", "-root", "_NET_CLIENT_LIST_STACKING").Output()
	if err != nil {
		return false
	}
	for _, id := range regexp.MustCompile(`0x[0-9a-fA-F]+`).FindAllString(string(data), -1) {
		value, err := exec.Command("xprop", "-id", id, "_NET_WM_PID").Output()
		if err != nil {
			continue
		}
		_, raw, ok := strings.Cut(string(value), "=")
		if ok && strings.TrimSpace(raw) == strconv.Itoa(pid) {
			return true
		}
	}
	return false
}
