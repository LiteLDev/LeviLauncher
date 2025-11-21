//go:build linux

package registry

type AppxInfo struct {
    PackageFullName   string `json:"PackageFullName"`
    PackageFamilyName string `json:"PackageFamilyName"`
    Version           string `json:"Version"`
    InstallLocation   string `json:"InstallLocation"`
}

func GetAppxInfo(packageName string) (*AppxInfo, error) {
    return nil, ErrNotSupported
}

var ErrNotSupported = errNotSupported{}

type errNotSupported struct{}

func (e errNotSupported) Error() string { return "not supported" }