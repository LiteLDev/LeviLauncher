package uwp

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	ReleasePackageName = "Microsoft.MinecraftUWP"
	PreviewPackageName = "Microsoft.MinecraftWindowsBeta"
	ReleaseFamilyName  = "Microsoft.MinecraftUWP_8wekyb3d8bbwe"
	PreviewFamilyName  = "Microsoft.MinecraftWindowsBeta_8wekyb3d8bbwe"
	microsoftPublisher = "CN=Microsoft Corporation, O=Microsoft Corporation, L=Redmond, S=Washington, C=US"
)

type Identity struct {
	Name         string `xml:"Name,attr"`
	Publisher    string `xml:"Publisher,attr"`
	Version      string `xml:"Version,attr"`
	Architecture string `xml:"ProcessorArchitecture,attr"`
}

type Application struct {
	ID         string `xml:"Id,attr"`
	Executable string `xml:"Executable,attr"`
}

type Manifest struct {
	XMLName      xml.Name      `xml:"Package"`
	Identity     Identity      `xml:"Identity"`
	Applications []Application `xml:"Applications>Application"`
}

type Error struct {
	Code  string
	Cause error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Code
	}
	return e.Code + ": " + e.Cause.Error()
}
func (e *Error) Unwrap() error             { return e.Cause }
func failure(code string, err error) error { return &Error{Code: code, Cause: err} }

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "ERR_UWP_INSTALL"
}

// ErrorMessage carries the stable code plus Windows diagnostics to the UI.
// Callers making control-flow decisions should continue to use ErrorCode.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Error()
	}
	return ErrorCode(err) + ": " + err.Error()
}

func ReadManifest(dir string) (Manifest, error) {
	var manifest Manifest
	f, err := os.Open(filepath.Join(dir, "AppxManifest.xml"))
	if err != nil {
		return manifest, failure("ERR_UWP_MANIFEST", err)
	}
	defer f.Close()
	if err := xml.NewDecoder(io.LimitReader(f, 4<<20)).Decode(&manifest); err != nil {
		return manifest, failure("ERR_UWP_MANIFEST", err)
	}
	if err := manifest.validate(); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func (m Manifest) validate() error {
	if !strings.EqualFold(m.Identity.Name, ReleasePackageName) && !strings.EqualFold(m.Identity.Name, PreviewPackageName) {
		return failure("ERR_UWP_NOT_MINECRAFT", fmt.Errorf("unsupported package identity %q", m.Identity.Name))
	}
	if !strings.EqualFold(strings.TrimSpace(m.Identity.Publisher), microsoftPublisher) {
		return failure("ERR_UWP_NOT_MINECRAFT", fmt.Errorf("unexpected Minecraft publisher"))
	}
	parts := strings.Split(m.Identity.Version, ".")
	if len(parts) != 4 {
		return failure("ERR_UWP_MANIFEST", fmt.Errorf("invalid package version"))
	}
	for _, p := range parts {
		if _, err := strconv.ParseUint(p, 10, 16); err != nil {
			return failure("ERR_UWP_MANIFEST", err)
		}
	}
	if len(m.Applications) == 0 {
		return failure("ERR_UWP_MANIFEST", fmt.Errorf("no application"))
	}
	app := m.Applications[0]
	if strings.TrimSpace(app.ID) == "" || strings.ContainsAny(app.ID, "!\\/\x00") {
		return failure("ERR_UWP_MANIFEST", fmt.Errorf("invalid application ID"))
	}
	if _, err := safeRelativePath(app.Executable); err != nil {
		return failure("ERR_UWP_MANIFEST", err)
	}
	if !strings.EqualFold(app.Executable, "Minecraft.Windows.exe") && !strings.EqualFold(app.Executable, "Minecraft.Win10.DX11.exe") {
		return failure("ERR_UWP_NOT_MINECRAFT", fmt.Errorf("unexpected Minecraft executable"))
	}
	return nil
}

func (m Manifest) IsPreview() bool { return strings.EqualFold(m.Identity.Name, PreviewPackageName) }
func (m Manifest) FamilyName() string {
	if m.IsPreview() {
		return PreviewFamilyName
	}
	return ReleaseFamilyName
}

// Early 0.x packages encode minor*10 + patch in the second component:
// 0.140.1.0 is catalog version 0.14.0.1. From 1.x onwards the third
// component encodes patch*100 + revision: 1.21.9301.0 is 1.21.93.1.
func (m Manifest) GameVersion() string {
	parts := strings.Split(m.Identity.Version, ".")
	if len(parts) != 4 {
		return ""
	}
	var numbers [4]int
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || value > 65535 {
			return ""
		}
		numbers[i] = value
	}
	if numbers[0] == 0 && numbers[1] >= 100 {
		return fmt.Sprintf("0.%d.%d.%d", numbers[1]/10, numbers[1]%10, numbers[2])
	}
	return fmt.Sprintf("%d.%d.%d.%d", numbers[0], numbers[1], numbers[2]/100, numbers[2]%100)
}
