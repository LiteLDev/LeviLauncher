package registry

import (
	"fmt"
	"log"
	"os/exec"
	"syscall"

	json "github.com/goccy/go-json"
)

type AppxInfo struct {
	PackageFullName   string `json:"PackageFullName"`
	PackageFamilyName string `json:"PackageFamilyName"`
	Version           string `json:"Version"`
	InstallLocation   string `json:"InstallLocation"`
}

func GetAppxInfo(packageName string) (*AppxInfo, error) {
	ps := "Get-AppxPackage -Name '" + packageName + "' | Select-Object PackageFullName, PackageFamilyName, Version, InstallLocation | ConvertTo-Json"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("GetAppxInfo error:", err, string(output))
		return nil, err
	}
	if len(output) == 0 {
		return nil, fmt.Errorf("empty result from Get-AppxPackage")
	}
	var info AppxInfo
	if err := json.Unmarshal(output, &info); err != nil {
		var arr []AppxInfo
		if err2 := json.Unmarshal(output, &arr); err2 == nil && len(arr) > 0 {
			info = arr[0]
		} else {
			log.Println("GetAppxInfo unmarshal error:", err)
			return nil, err
		}
	}
	if info.InstallLocation == "" || info.PackageFullName == "" {
		return nil, fmt.Errorf("package %s not found", packageName)
	}
	return &info, nil
}
