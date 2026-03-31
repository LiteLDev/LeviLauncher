package gdk

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"text/template"

	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

var appxManifestTemplate = template.Must(template.New("appxManifest").Funcs(template.FuncMap{
	"attr": xmlEscapeAttr,
	"text": xmlEscapeText,
}).Parse(`<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10" xmlns:desktop6="http://schemas.microsoft.com/appx/manifest/desktop/windows10/6" xmlns:desktop="http://schemas.microsoft.com/appx/manifest/desktop/windows10" xmlns:uap3="http://schemas.microsoft.com/appx/manifest/uap/windows10/3" xmlns:wincap="http://schemas.microsoft.com/appx/manifest/foundation/windows10/windowscapabilities" xmlns:rescap="http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities" IgnorableNamespaces="uap uap3 desktop desktop6 wincap rescap" xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10">
  <Identity Name="{{attr .IdentityName}}" Publisher="{{attr .Publisher}}" Version="{{attr .Version}}" ProcessorArchitecture="{{attr .ProcessorArchitecture}}" />
  <Properties>
    <DisplayName>{{text .DisplayName}}</DisplayName>
    <PublisherDisplayName>{{text .PublisherDisplayName}}</PublisherDisplayName>
    <Logo>{{text .StoreLogo}}</Logo>
    <Description>{{text .Description}}</Description>
    <desktop6:RegistryWriteVirtualization>disabled</desktop6:RegistryWriteVirtualization>
    <desktop6:FileSystemWriteVirtualization>disabled</desktop6:FileSystemWriteVirtualization>
  </Properties>
  <Dependencies>
    <TargetDeviceFamily Name="Windows.Desktop" MinVersion="10.0.18362.0" MaxVersionTested="10.0.18362.0" />
{{- range .PackageDependencies }}
    <PackageDependency Name="{{attr .Name}}" MinVersion="{{attr .MinVersion}}" Publisher="{{attr .Publisher}}" />
{{- end }}
  </Dependencies>
  <Resources>
{{- range .Resources }}
    <Resource Language="{{attr .}}" />
{{- end }}
  </Resources>
  <Applications>
    <Application Id="{{attr .ApplicationID}}" Executable="{{attr .Executable}}" EntryPoint="Windows.FullTrustApplication">
      <uap:VisualElements DisplayName="{{attr .DisplayName}}" Square150x150Logo="{{attr .Square150x150Logo}}" Square44x44Logo="{{attr .Square44x44Logo}}" Description="{{attr .Description}}" ForegroundText="{{attr .ForegroundText}}" BackgroundColor="{{attr .BackgroundColor}}">
        <uap:SplashScreen Image="{{attr .SplashScreenImage}}" />
      </uap:VisualElements>
      <Extensions>
{{- range .Protocols }}
        <uap:Extension Category="windows.protocol">
          <uap:Protocol Name="{{attr .}}" />
        </uap:Extension>
{{- end }}
{{- range .FileTypeAssociations }}
        <uap:Extension Category="windows.fileTypeAssociation">
          <uap:FileTypeAssociation Name="{{attr .Name}}">
            <uap:DisplayName>{{text .DisplayName}}</uap:DisplayName>
{{- if .InfoTip }}
            <uap:InfoTip>{{text .InfoTip}}</uap:InfoTip>
{{- end }}
            <uap:EditFlags OpenIsSafe="{{attr .OpenIsSafe}}" AlwaysUnsafe="{{attr .AlwaysUnsafe}}" />
            <uap:SupportedFileTypes>
{{- range .SupportedFileTypes }}
              <uap:FileType>{{text .}}</uap:FileType>
{{- end }}
            </uap:SupportedFileTypes>
          </uap:FileTypeAssociation>
        </uap:Extension>
{{- end }}
      </Extensions>
    </Application>
  </Applications>
  <Capabilities>
    <Capability Name="internetClient" />
    <rescap:Capability Name="runFullTrust" />
    <rescap:Capability Name="appLicensing" />
    <rescap:Capability Name="unvirtualizedResources" />
  </Capabilities>
</Package>
`))

type microsoftGameConfig struct {
	Identity            microsoftGameIdentity            `xml:"Identity"`
	TitleID             string                           `xml:"TitleId"`
	ShellVisuals        microsoftGameShellVisuals        `xml:"ShellVisuals"`
	Resources           []microsoftGameResource          `xml:"Resources>Resource"`
	Executables         []microsoftGameExecutable        `xml:"ExecutableList>Executable"`
	Protocols           []microsoftGameProtocol          `xml:"ProtocolList>Protocol"`
	DesktopRegistration microsoftGameDesktopRegistration `xml:"DesktopRegistration"`
}

type microsoftGameIdentity struct {
	Name      string `xml:"Name,attr"`
	Publisher string `xml:"Publisher,attr"`
	Version   string `xml:"Version,attr"`
}

type microsoftGameShellVisuals struct {
	DefaultDisplayName   string `xml:"DefaultDisplayName,attr"`
	PublisherDisplayName string `xml:"PublisherDisplayName,attr"`
	StoreLogo            string `xml:"StoreLogo,attr"`
	Square150x150Logo    string `xml:"Square150x150Logo,attr"`
	Square44x44Logo      string `xml:"Square44x44Logo,attr"`
	Description          string `xml:"Description,attr"`
	ForegroundText       string `xml:"ForegroundText,attr"`
	BackgroundColor      string `xml:"BackgroundColor,attr"`
	SplashScreenImage    string `xml:"SplashScreenImage,attr"`
}

type microsoftGameResource struct {
	Language string `xml:"Language,attr"`
}

type microsoftGameExecutable struct {
	Name               string `xml:"Name,attr"`
	TargetDeviceFamily string `xml:"TargetDeviceFamily,attr"`
	ID                 string `xml:"Id,attr"`
}

type microsoftGameProtocol struct {
	Name string `xml:"Name,attr"`
}

type microsoftGameDesktopRegistration struct {
	MultiplayerProtocol  bool                               `xml:"MultiplayerProtocol"`
	DependencyList       microsoftGameDependencyList        `xml:"DependencyList"`
	FileTypeAssociations []microsoftGameFileTypeAssociation `xml:"FileTypeAssociation"`
}

type microsoftGameDependencyList struct {
	KnownDependencies []microsoftGameKnownDependency `xml:"KnownDependency"`
}

type microsoftGameKnownDependency struct {
	Name string `xml:"Name,attr"`
}

type microsoftGameFileTypeAssociation struct {
	Name               string                           `xml:"Name,attr"`
	DisplayName        string                           `xml:"DisplayName"`
	InfoTip            string                           `xml:"InfoTip"`
	EditFlags          microsoftGameEditFlags           `xml:"EditFlags"`
	SupportedFileTypes []microsoftGameSupportedFileType `xml:"SupportedFileTypes>FileType"`
}

type microsoftGameEditFlags struct {
	OpenIsSafe   string `xml:"OpenIsSafe,attr"`
	AlwaysUnsafe string `xml:"AlwaysUnsafe,attr"`
}

type microsoftGameSupportedFileType struct {
	Value string `xml:",chardata"`
}

type appxPackageDependency struct {
	Name       string
	MinVersion string
	Publisher  string
}

type appxFileTypeAssociation struct {
	Name               string
	DisplayName        string
	InfoTip            string
	OpenIsSafe         string
	AlwaysUnsafe       string
	SupportedFileTypes []string
}

type appxManifestData struct {
	IdentityName          string
	Publisher             string
	Version               string
	ProcessorArchitecture string
	DisplayName           string
	PublisherDisplayName  string
	StoreLogo             string
	Description           string
	Square150x150Logo     string
	Square44x44Logo       string
	ForegroundText        string
	BackgroundColor       string
	SplashScreenImage     string
	ApplicationID         string
	Executable            string
	Resources             []string
	Protocols             []string
	PackageDependencies   []appxPackageDependency
	FileTypeAssociations  []appxFileTypeAssociation
}

func wdappPath() string {
	return `C:\Program Files (x86)\Microsoft GDK\bin\wdapp.exe`
}

func WdappExists() bool { return utils.FileExists(wdappPath()) }

func logCommandOutput(scope string, output []byte) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return
	}
	log.Printf("%s output: %s", scope, trimmed)
}

func runHiddenCommand(scope string, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	logCommandOutput(scope, output)
	if err != nil {
		log.Printf("%s failed: %v", scope, err)
		return err
	}
	log.Printf("%s succeeded", scope)
	return nil
}

func normalizeInstallPath(path string) string {
	norm := strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
	norm = strings.TrimPrefix(norm, `\\?\`)
	norm = strings.TrimPrefix(norm, `\??\`)
	return norm
}

func xmlEscapeAttr(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	escaped := buf.String()
	escaped = strings.ReplaceAll(escaped, `"`, "&quot;")
	escaped = strings.ReplaceAll(escaped, "\r", "&#xD;")
	escaped = strings.ReplaceAll(escaped, "\n", "&#xA;")
	return escaped
}

func xmlEscapeText(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	return buf.String()
}

func architectureForManifest() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	case "arm64":
		return "arm64"
	case "arm":
		return "arm"
	default:
		return runtime.GOARCH
	}
}

func packageDependenciesFromConfig(cfg *microsoftGameConfig) []appxPackageDependency {
	deps := make([]appxPackageDependency, 0, len(cfg.DesktopRegistration.DependencyList.KnownDependencies))
	for _, dependency := range cfg.DesktopRegistration.DependencyList.KnownDependencies {
		switch strings.TrimSpace(dependency.Name) {
		case "VC14":
			deps = append(deps, appxPackageDependency{
				Name:       "Microsoft.VCLibs.140.00.UWPDesktop",
				MinVersion: "14.0.33728.0",
				Publisher:  "CN=Microsoft Corporation, O=Microsoft Corporation, L=Redmond, S=Washington, C=US",
			})
		case "":
		default:
			log.Printf("gdk.generateAppxManifest: ignoring unsupported known dependency %q", dependency.Name)
		}
	}
	return deps
}

func protocolsFromConfig(cfg *microsoftGameConfig) []string {
	seen := make(map[string]struct{})
	protocols := make([]string, 0, len(cfg.Protocols)+2)
	appendUnique := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		protocols = append(protocols, value)
	}
	if cfg.DesktopRegistration.MultiplayerProtocol {
		if titleID := strings.ToLower(strings.TrimSpace(cfg.TitleID)); titleID != "" {
			appendUnique("ms-xbl-" + titleID)
		}
		appendUnique("ms-xbl-multiplayer")
	}
	for _, protocol := range cfg.Protocols {
		appendUnique(protocol.Name)
	}
	return protocols
}

func fileTypeAssociationsFromConfig(cfg *microsoftGameConfig) []appxFileTypeAssociation {
	associations := make([]appxFileTypeAssociation, 0, len(cfg.DesktopRegistration.FileTypeAssociations))
	for _, assoc := range cfg.DesktopRegistration.FileTypeAssociations {
		fileTypes := make([]string, 0, len(assoc.SupportedFileTypes))
		for _, fileType := range assoc.SupportedFileTypes {
			value := strings.TrimSpace(fileType.Value)
			if value != "" {
				fileTypes = append(fileTypes, value)
			}
		}
		associations = append(associations, appxFileTypeAssociation{
			Name:               strings.TrimSpace(assoc.Name),
			DisplayName:        strings.TrimSpace(assoc.DisplayName),
			InfoTip:            strings.TrimSpace(assoc.InfoTip),
			OpenIsSafe:         strings.TrimSpace(assoc.EditFlags.OpenIsSafe),
			AlwaysUnsafe:       strings.TrimSpace(assoc.EditFlags.AlwaysUnsafe),
			SupportedFileTypes: fileTypes,
		})
	}
	return associations
}

func buildAppxManifestData(cfg *microsoftGameConfig) (*appxManifestData, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(cfg.Identity.Name) == "" || strings.TrimSpace(cfg.Identity.Publisher) == "" || strings.TrimSpace(cfg.Identity.Version) == "" {
		return nil, fmt.Errorf("identity fields are incomplete")
	}
	if strings.TrimSpace(cfg.ShellVisuals.DefaultDisplayName) == "" || strings.TrimSpace(cfg.ShellVisuals.PublisherDisplayName) == "" {
		return nil, fmt.Errorf("shell visuals are incomplete")
	}
	var executable microsoftGameExecutable
	for _, candidate := range cfg.Executables {
		if strings.EqualFold(strings.TrimSpace(candidate.TargetDeviceFamily), "PC") && strings.TrimSpace(candidate.Name) != "" && strings.TrimSpace(candidate.ID) != "" {
			executable = candidate
			break
		}
	}
	if strings.TrimSpace(executable.Name) == "" || strings.TrimSpace(executable.ID) == "" {
		return nil, fmt.Errorf("no PC executable found in MicrosoftGame.Config")
	}
	resources := make([]string, 0, len(cfg.Resources))
	for _, resource := range cfg.Resources {
		if language := strings.TrimSpace(resource.Language); language != "" {
			resources = append(resources, language)
		}
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("no resources found in MicrosoftGame.Config")
	}
	data := &appxManifestData{
		IdentityName:          strings.TrimSpace(cfg.Identity.Name),
		Publisher:             strings.TrimSpace(cfg.Identity.Publisher),
		Version:               strings.TrimSpace(cfg.Identity.Version),
		ProcessorArchitecture: architectureForManifest(),
		DisplayName:           strings.TrimSpace(cfg.ShellVisuals.DefaultDisplayName),
		PublisherDisplayName:  strings.TrimSpace(cfg.ShellVisuals.PublisherDisplayName),
		StoreLogo:             strings.TrimSpace(cfg.ShellVisuals.StoreLogo),
		Description:           strings.TrimSpace(cfg.ShellVisuals.Description),
		Square150x150Logo:     strings.TrimSpace(cfg.ShellVisuals.Square150x150Logo),
		Square44x44Logo:       strings.TrimSpace(cfg.ShellVisuals.Square44x44Logo),
		ForegroundText:        strings.TrimSpace(cfg.ShellVisuals.ForegroundText),
		BackgroundColor:       strings.TrimSpace(cfg.ShellVisuals.BackgroundColor),
		SplashScreenImage:     strings.TrimSpace(cfg.ShellVisuals.SplashScreenImage),
		ApplicationID:         strings.TrimSpace(executable.ID),
		Executable:            strings.TrimSpace(executable.Name),
		Resources:             resources,
		Protocols:             protocolsFromConfig(cfg),
		PackageDependencies:   packageDependenciesFromConfig(cfg),
		FileTypeAssociations:  fileTypeAssociationsFromConfig(cfg),
	}
	if data.Description == "" {
		data.Description = data.DisplayName
	}
	if data.ForegroundText == "" {
		data.ForegroundText = "light"
	}
	if data.BackgroundColor == "" {
		data.BackgroundColor = "#000000"
	}
	if data.StoreLogo == "" || data.Square150x150Logo == "" || data.Square44x44Logo == "" || data.SplashScreenImage == "" {
		return nil, fmt.Errorf("required visual assets are incomplete")
	}
	return data, nil
}

func readMicrosoftGameConfig(folder string) (*microsoftGameConfig, error) {
	configPath := filepath.Join(folder, "MicrosoftGame.Config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg microsoftGameConfig
	if err := xml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateManifestInputs(folder string, manifest *appxManifestData) error {
	requiredFiles := []string{
		manifest.Executable,
		manifest.StoreLogo,
		manifest.Square150x150Logo,
		manifest.Square44x44Logo,
		manifest.SplashScreenImage,
		"resources.pri",
	}
	for _, name := range requiredFiles {
		path := filepath.Join(folder, name)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("required file missing: %s", path)
		}
		if info.IsDir() {
			return fmt.Errorf("required file is a directory: %s", path)
		}
	}
	return nil
}

func renderAppxManifest(manifest *appxManifestData) ([]byte, error) {
	var buf bytes.Buffer
	if err := appxManifestTemplate.Execute(&buf, manifest); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func createRegistrationManifest(folder string, content []byte) (string, func(), error) {
	path := filepath.Join(folder, "AppXManifest.xml")
	var originalContent []byte
	var originalMode os.FileMode
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if info.IsDir() {
			return "", nil, fmt.Errorf("%s is a directory", path)
		}
		originalContent, err = os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		originalMode = info.Mode().Perm()
	case os.IsNotExist(err):
		originalMode = 0o644
	default:
		return "", nil, err
	}
	if err := os.WriteFile(path, content, originalMode); err != nil {
		return "", nil, err
	}
	cleanup := func() {
		if originalContent != nil {
			if err := os.WriteFile(path, originalContent, originalMode); err != nil {
				log.Printf("gdk.createRegistrationManifest: failed to restore manifest %s: %v", path, err)
			}
			return
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("gdk.createRegistrationManifest: failed to remove manifest %s: %v", path, err)
		}
	}
	return path, cleanup, nil
}

func quotePowerShellSingle(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func registerLoosePackageWithPowerShell(manifestPath string) error {
	command := "Add-AppxPackage -ForceApplicationShutdown -Register " + quotePowerShellSingle(manifestPath)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	return runHiddenCommand("gdk.RegisterVersionFolder: Add-AppxPackage", cmd)
}

func registerVersionFolderInternal(folder string) error {
	cfg, err := readMicrosoftGameConfig(folder)
	if err != nil {
		return err
	}
	manifest, err := buildAppxManifestData(cfg)
	if err != nil {
		return err
	}
	if err := validateManifestInputs(folder, manifest); err != nil {
		return err
	}
	content, err := renderAppxManifest(manifest)
	if err != nil {
		return err
	}
	manifestPath, cleanup, err := createRegistrationManifest(folder, content)
	if err != nil {
		return err
	}
	defer cleanup()
	log.Printf("gdk.RegisterVersionFolder: generated registration manifest %s", manifestPath)
	return registerLoosePackageWithPowerShell(manifestPath)
}

func RegisterVersionFolder(folder string) string {
	folder = filepath.Clean(strings.TrimSpace(folder))
	log.Printf("gdk.RegisterVersionFolder: start folder=%s", folder)
	if folder == "" || !utils.FileExists(folder) {
		log.Printf("gdk.RegisterVersionFolder: target folder missing or empty: %s", folder)
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	if !registry.IsDevModeEnabled() {
		log.Printf("gdk.RegisterVersionFolder: Windows Developer Mode disabled, trying to enable it")
		if !registry.TryEnableDevMode() {
			log.Printf("gdk.RegisterVersionFolder: failed to enable Windows Developer Mode")
			return "ERR_DEV_MODE_REQUIRED"
		}
		log.Printf("gdk.RegisterVersionFolder: Windows Developer Mode enabled")
	}
	if err := registerVersionFolderInternal(folder); err != nil {
		log.Printf("gdk.RegisterVersionFolder: internal register failed for folder=%s: %v", folder, err)
		return "ERR_REGISTER_FAILED"
	}
	log.Printf("gdk.RegisterVersionFolder: completed folder=%s", folder)
	return ""
}

func UnregisterIfExists(isPreview bool) string {
	pkg := "MICROSOFT.MINECRAFTUWP"
	if isPreview {
		pkg = "Microsoft.MinecraftWindowsBeta"
	}
	log.Printf("gdk.UnregisterIfExists: checking existing package=%s", pkg)
	if info, err := registry.GetAppxInfo(pkg); err == nil && info != nil {
		pf := strings.TrimSpace(info.PackageFullName)
		log.Printf("gdk.UnregisterIfExists: found package=%s fullName=%s installLocation=%s", pkg, pf, strings.TrimSpace(info.InstallLocation))
		if pf != "" {
			cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Remove-AppxPackage -Package '"+pf+"' -PreserveRoamableApplicationData")
			log.Printf("gdk.UnregisterIfExists: removing package=%s fullName=%s", pkg, pf)
			if er := runHiddenCommand("gdk.UnregisterIfExists: Remove-AppxPackage", cmd); er != nil {
				return "ERR_UNREGISTER_FAILED"
			}
		}
	} else if err != nil {
		log.Printf("gdk.UnregisterIfExists: GetAppxInfo failed for package=%s: %v", pkg, err)
	} else {
		log.Printf("gdk.UnregisterIfExists: package=%s not found", pkg)
	}
	return ""
}

func UnregisterVersionFolder(folder string) string {
	folder = filepath.Clean(strings.TrimSpace(folder))
	log.Printf("gdk.UnregisterVersionFolder: start folder=%s", folder)
	if folder == "" {
		log.Printf("gdk.UnregisterVersionFolder: target folder is empty")
		return "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	check := func(pkg string) string {
		info, err := registry.GetAppxInfo(pkg)
		if err != nil || info == nil {
			if err != nil {
				log.Printf("gdk.UnregisterVersionFolder: GetAppxInfo failed for package=%s: %v", pkg, err)
			}
			return ""
		}
		loc := normalizeInstallPath(info.InstallLocation)
		f := normalizeInstallPath(folder)
		log.Printf("gdk.UnregisterVersionFolder: compare package=%s installLocation=%s target=%s", pkg, loc, f)
		if loc == f {
			log.Printf("gdk.UnregisterVersionFolder: matched package=%s fullName=%s", pkg, strings.TrimSpace(info.PackageFullName))
			return strings.TrimSpace(info.PackageFullName)
		}
		return ""
	}
	pf := check("MICROSOFT.MINECRAFTUWP")
	if pf == "" {
		pf = check("Microsoft.MinecraftWindowsBeta")
	}
	if pf == "" {
		log.Printf("gdk.UnregisterVersionFolder: folder=%s is not registered to system", folder)
		return "ERR_NOT_REGISTERED_THIS_VERSION"
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Remove-AppxPackage -Package '"+pf+"' -PreserveRoamableApplicationData")
	log.Printf("gdk.UnregisterVersionFolder: removing fullName=%s", pf)
	if er := runHiddenCommand("gdk.UnregisterVersionFolder: Remove-AppxPackage", cmd); er != nil {
		return "ERR_UNREGISTER_FAILED"
	}
	log.Printf("gdk.UnregisterVersionFolder: completed folder=%s", folder)
	return ""
}
