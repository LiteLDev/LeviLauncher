package uwp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

type registeredPackage struct {
	PackageFullName   string
	PackageFamilyName string
	InstallLocation   string
	IsDevelopmentMode bool
}

var deploymentMu sync.Mutex
var runPowerShell = executePowerShell
var activateApplication = activate
var activateProtocol = activateWithURI

func executePowerShell(ctx context.Context, script string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// EncodedCommand avoids command-line quoting and preserves Unicode paths.
	script = "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue'; [Console]::OutputEncoding = [System.Text.Encoding]::UTF8; try { " + script + " } catch { [Console]::Error.WriteLine(($_ | Out-String)); exit 1 }"
	u16 := utf16.Encode([]rune(script))
	data := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(data[i*2:], v)
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-OutputFormat", "Text", "-EncodedCommand", base64.StdEncoding.EncodeToString(data))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		// Deployment failures retain their diagnostics, but successful queries
		// must not parse PowerShell's stderr progress/CLIXML as package JSON.
		return append(stdout.Bytes(), stderr.Bytes()...), err
	}
	return stdout.Bytes(), nil
}

func psQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

func queryPackage(ctx context.Context, name string) (*registeredPackage, error) {
	output, err := runPowerShell(ctx, "Get-AppxPackage -Name "+psQuote(name)+" | Select-Object -First 1 PackageFullName,PackageFamilyName,InstallLocation,IsDevelopmentMode | ConvertTo-Json -Compress")
	if err != nil {
		return nil, failure("ERR_UWP_QUERY", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output))))
	}
	if strings.TrimSpace(string(output)) == "" || strings.TrimSpace(string(output)) == "null" {
		return nil, nil
	}
	var pkg registeredPackage
	if err := json.Unmarshal(output, &pkg); err != nil {
		return nil, failure("ERR_UWP_QUERY", err)
	}
	return &pkg, nil
}

func canonicalPath(value string) string {
	value = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), `\\?\`), `\??\`)
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		value = resolved
	}
	abs, err := filepath.Abs(value)
	if err == nil {
		value = abs
	}
	return strings.ToLower(filepath.Clean(value))
}

func samePath(a, b string) bool {
	return strings.TrimSpace(a) != "" && strings.TrimSpace(b) != "" && canonicalPath(a) == canonicalPath(b)
}

func isManagedDevelopmentPackage(pkg *registeredPackage) bool {
	if pkg == nil || !pkg.IsDevelopmentMode || strings.TrimSpace(pkg.InstallLocation) == "" {
		return false
	}
	return isManagedVersionDirectory(pkg.InstallLocation)
}

func isManagedVersionDirectory(dir string) bool {
	root, err := apppath.VersionsDir()
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(canonicalPath(root), canonicalPath(dir))
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Store-origin signatures cannot be used for loose developer registration.
// Keep the original signature alongside the instance, and leave the downloaded
// archive intact. This also repairs instances extracted by earlier versions.
func prepareLooseRegistration(dir string) error {
	if !isManagedVersionDirectory(dir) {
		return failure("ERR_UWP_PACKAGE_CONFLICT", fmt.Errorf("loose registration requires a managed version directory"))
	}
	signature := filepath.Join(dir, "AppxSignature.p7x")
	info, err := os.Lstat(signature)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return failure("ERR_UWP_PREPARE", err)
	}
	if !info.Mode().IsRegular() {
		return failure("ERR_UWP_PREPARE", fmt.Errorf("package signature is not a regular file"))
	}
	backup := signature + ".levilauncher-backup"
	if _, err := os.Lstat(backup); err == nil {
		original, readErr := os.ReadFile(signature)
		saved, backupErr := os.ReadFile(backup)
		if readErr != nil || backupErr != nil || !bytes.Equal(original, saved) {
			return failure("ERR_UWP_PREPARE", fmt.Errorf("existing signature backup differs from the package signature"))
		}
		if err := os.Remove(signature); err != nil {
			return failure("ERR_UWP_PREPARE", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return failure("ERR_UWP_PREPARE", err)
	}
	if err := os.Rename(signature, backup); err != nil {
		return failure("ERR_UWP_PREPARE", err)
	}
	return nil
}

func deploymentError(defaultCode string, output []byte, err error) error {
	message := strings.TrimSpace(string(output))
	lower := strings.ToLower(message)
	code := defaultCode
	switch {
	case strings.Contains(lower, "0x80073cf3"):
		code = "ERR_UWP_DEPENDENCY"
	case strings.Contains(lower, "0x80073cfd"):
		code = "ERR_UWP_SYSTEM_VERSION"
	case strings.Contains(lower, "0x80073cff"), strings.Contains(lower, "0x80073d01"):
		code = "ERR_UWP_DEVELOPER_MODE"
	case strings.Contains(lower, "0x80073d02"):
		code = "ERR_GAME_ALREADY_RUNNING"
	case strings.Contains(lower, "0x80073cfb"), strings.Contains(lower, "0x80073d06"):
		code = "ERR_UWP_PACKAGE_CONFLICT"
	}
	if err == context.Canceled {
		code = "ERR_CANCELED"
	}
	return failure(code, fmt.Errorf("%w: %s", err, message))
}

func addPackage(ctx context.Context, dir string) error {
	// Register the loose files in development mode. ForceUpdateFromAnyVersion
	// permits downgrades through the Windows deployment transaction, preserving
	// the existing package data without first uninstalling a Store package.
	return withFullTrustManifest(dir, func() error {
		out, err := runPowerShell(ctx, "Add-AppxPackage -Register "+psQuote(filepath.Join(dir, "AppxManifest.xml"))+" -ForceUpdateFromAnyVersion")
		if err != nil {
			return deploymentError("ERR_UWP_REGISTER", out, err)
		}
		return nil
	})
}

func removeDevelopmentPackage(ctx context.Context, pkg *registeredPackage) error {
	if !isManagedDevelopmentPackage(pkg) {
		return failure("ERR_UWP_PACKAGE_CONFLICT", fmt.Errorf("existing package is not a managed development registration"))
	}
	// PreserveApplicationData is supported only for development registrations;
	// PreserveRoamableApplicationData alone does NOT preserve LocalState worlds.
	out, err := runPowerShell(ctx, "Remove-AppxPackage -Package "+psQuote(pkg.PackageFullName)+" -PreserveApplicationData")
	if err != nil {
		return deploymentError("ERR_UWP_UNREGISTER", out, err)
	}
	return nil
}

func registerLocked(ctx context.Context, dir string, manifest Manifest) (*registeredPackage, error) {
	existing, err := queryPackage(ctx, manifest.Identity.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil && !strings.EqualFold(existing.PackageFamilyName, manifest.FamilyName()) {
		return nil, failure("ERR_UWP_PACKAGE_CONFLICT", fmt.Errorf("existing package publisher does not match Minecraft"))
	}
	// Explicit registration also upgrades an already registered instance from
	// AppContainer to full trust; its install location alone cannot prove that.
	if err := prepareLooseRegistration(dir); err != nil {
		return nil, err
	}
	if err = addPackage(ctx, dir); err != nil {
		if !isManagedDevelopmentPackage(existing) {
			if existing != nil && ErrorCode(err) == "ERR_UWP_REGISTER" {
				return nil, failure("ERR_UWP_PACKAGE_CONFLICT", err)
			}
			return nil, err
		}
		// Only a conflict warrants unregistering. Missing dependencies, disabled
		// developer mode and a running game must leave the old registration alone.
		if ErrorCode(err) != "ERR_UWP_PACKAGE_CONFLICT" {
			return nil, err
		}
		if info, readErr := os.Stat(filepath.Join(existing.InstallLocation, "AppxManifest.xml")); readErr != nil || !info.Mode().IsRegular() {
			return nil, failure("ERR_UWP_PACKAGE_CONFLICT", fmt.Errorf("existing registration cannot be restored because its manifest is missing"))
		}
		if removeErr := removeDevelopmentPackage(ctx, existing); removeErr != nil {
			return nil, removeErr
		}
		if err = addPackage(ctx, dir); err != nil {
			// Use an independent context for rollback even if launch was cancelled.
			if rollbackErr := addPackage(context.Background(), existing.InstallLocation); rollbackErr != nil {
				return nil, failure("ERR_UWP_REGISTER_ROLLBACK", fmt.Errorf("registration: %v; rollback: %w", err, rollbackErr))
			}
			return nil, err
		}
	}
	registered, err := queryPackage(ctx, manifest.Identity.Name)
	if err != nil {
		return nil, err
	}
	if registered == nil || !samePath(registered.InstallLocation, dir) || !registered.IsDevelopmentMode || !strings.EqualFold(registered.PackageFamilyName, manifest.FamilyName()) {
		return nil, failure("ERR_UWP_REGISTER", fmt.Errorf("registered package does not match the selected instance"))
	}
	return registered, nil
}

func Register(ctx context.Context, dir string) error {
	manifest, err := ReadManifest(dir)
	if err != nil {
		return err
	}
	deploymentMu.Lock()
	defer deploymentMu.Unlock()
	_, err = registerLocked(ctx, dir, manifest)
	return err
}

func Unregister(ctx context.Context, dir string) error {
	manifest, err := ReadManifest(dir)
	if err != nil {
		return err
	}
	deploymentMu.Lock()
	defer deploymentMu.Unlock()
	pkg, err := queryPackage(ctx, manifest.Identity.Name)
	if err != nil {
		return err
	}
	if pkg == nil || !samePath(pkg.InstallLocation, dir) {
		return nil
	}
	if !strings.EqualFold(pkg.PackageFamilyName, manifest.FamilyName()) {
		return failure("ERR_UWP_PACKAGE_CONFLICT", fmt.Errorf("registered package publisher does not match Minecraft"))
	}
	return removeDevelopmentPackage(ctx, pkg)
}

func Launch(ctx context.Context, dir string) (int, error) {
	return LaunchWithPreparation(ctx, dir, false, nil, nil)
}

// LaunchWithPreparation upgrades an already registered managed instance to
// full trust before preparing its native loader. It never registers an absent
// instance or switches the selected package as a side effect of launching.
// beforeActivate runs only after preparation succeeds, just before Windows
// activation. It lets callers observe a window before the activation API returns.
func LaunchWithPreparation(ctx context.Context, dir string, enableEditorMode bool, prepare func() error, beforeActivate func()) (int, error) {
	manifest, err := ReadManifest(dir)
	if err != nil {
		return 0, err
	}
	channel := "release"
	if strings.EqualFold(manifest.Identity.Name, PreviewPackageName) {
		channel = "preview"
	}
	// The actual package guards launches even if imported metadata was edited.
	enableEditorMode = enableEditorMode && versions.SupportsEditorMode(manifest.GameVersion(), channel)
	deploymentMu.Lock()
	defer deploymentMu.Unlock()
	pkg, err := queryPackage(ctx, manifest.Identity.Name)
	if err != nil {
		return 0, err
	}
	if pkg == nil || !samePath(pkg.InstallLocation, dir) || !strings.EqualFold(pkg.PackageFamilyName, manifest.FamilyName()) {
		return 0, failure("ERR_UWP_NOT_REGISTERED", fmt.Errorf("register the selected UWP instance before launching it"))
	}
	if prepare != nil {
		if !isManagedDevelopmentPackage(pkg) {
			return 0, failure("ERR_UWP_PACKAGE_CONFLICT", fmt.Errorf("native loading requires a managed development registration"))
		}
		// Re-apply registration so instances registered by older launchers do
		// not attempt to load desktop DLLs inside an AppContainer.
		if pkg, err = registerLocked(ctx, dir, manifest); err != nil {
			return 0, err
		}
		if err := prepare(); err != nil {
			return 0, failure("ERR_UWP_PREPARE", err)
		}
	}
	appID := pkg.PackageFamilyName + "!" + manifest.Applications[0].ID
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if beforeActivate != nil {
		beforeActivate()
	}
	var pid int
	if enableEditorMode {
		pid, err = activateProtocol(appID, versions.EditorLaunchURI(versions.PackageTypeUWP, channel))
	} else {
		pid, err = activateApplication(appID)
	}
	if err != nil {
		return 0, failure("ERR_UWP_LAUNCH", err)
	}
	return pid, nil
}
