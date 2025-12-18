package registry

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func IsDevModeEnabled() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue("AllowDevelopmentWithoutDevLicense")
	if err != nil {
		return false
	}
	return val == 1
}

func TryEnableDevMode() bool {
	psCommand := `Start-Process -FilePath "reg" -ArgumentList 'add', 'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock', '/v', 'AllowDevelopmentWithoutDevLicense', '/t', 'REG_DWORD', '/d', '1', '/f' -Verb RunAs -Wait`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()

	return IsDevModeEnabled()
}
