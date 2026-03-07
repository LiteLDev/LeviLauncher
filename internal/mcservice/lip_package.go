package mcservice

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

var lipIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

const leviLaminaClientPackageRefBase = "github.com/LiteLDev/LeviLamina#client"

func normalizeLIPPackageIdentifier(identifier string) string {
	return strings.TrimSpace(identifier)
}

func canonicalizeLIPPackageIdentifier(identifier string) string {
	normalized := normalizeLIPPackageIdentifier(identifier)
	if normalized == "" {
		return ""
	}

	if idx := strings.Index(normalized, "#"); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}
	lowerNormalized := strings.ToLower(normalized)
	if strings.HasPrefix(lowerNormalized, "https://") {
		normalized = normalized[len("https://"):]
	} else if strings.HasPrefix(lowerNormalized, "http://") {
		normalized = normalized[len("http://"):]
	}
	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return ""
	}

	parts := strings.FieldsFunc(normalized, func(r rune) bool { return r == '/' })
	if len(parts) == 2 {
		return strings.Join(parts, "/")
	}
	if len(parts) == 3 && strings.Contains(parts[0], ".") {
		return fmt.Sprintf("%s/%s", parts[1], parts[2])
	}

	return ""
}

func buildLIPPackageRefBase(identifier string) (string, string, bool) {
	normalizedIdentifier := canonicalizeLIPPackageIdentifier(identifier)
	if !lipIdentifierPattern.MatchString(normalizedIdentifier) {
		return "", "", false
	}
	return normalizedIdentifier, fmt.Sprintf("github.com/%s#client", normalizedIdentifier), true
}

func resolveLIPTargetDir(targetName string) (string, string) {
	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "", "ERR_ACCESS_VERSIONS_DIR"
	}

	normalizedTargetName := strings.TrimSpace(targetName)
	if normalizedTargetName == "" {
		return "", "ERR_TARGET_NOT_FOUND"
	}

	targetDir := filepath.Join(vdir, normalizedTargetName)
	if !utils.DirExists(targetDir) {
		return "", "ERR_TARGET_NOT_FOUND"
	}

	return targetDir, ""
}

func InstallLIPPackage(ctx context.Context, targetName string, identifier string, version string) string {
	if !lip.IsInstalled() {
		return "ERR_LIP_NOT_INSTALLED"
	}

	normalizedIdentifier, packageRefBase, ok := buildLIPPackageRefBase(identifier)
	if !ok {
		return "ERR_LIP_PACKAGE_INVALID_IDENTIFIER"
	}

	normalizedVersion := strings.TrimSpace(version)
	if normalizedVersion == "" {
		return "ERR_LIP_PACKAGE_VERSION_REQUIRED"
	}

	targetDir, errCode := resolveLIPTargetDir(targetName)
	if errCode != "" {
		return errCode
	}

	pkg := fmt.Sprintf("%s@%s", packageRefBase, normalizedVersion)
	targetOnlyPkgs := []string{pkg}
	installPkgs := []string{pkg}
	hasPinnedLL := false

	// Pin existing LeviLamina version to avoid dependency solver switching LL
	// or fetching a large set of LL candidates from wide ranges (e.g. 1.9.*).
	if !strings.EqualFold(packageRefBase, leviLaminaClientPackageRefBase) {
		llState, llQueryErr := lip.GetPackageInstallStateViaDaemon(ctx, targetDir, leviLaminaClientPackageRefBase)
		if llQueryErr != nil {
			if code := lip.ErrorCode(llQueryErr); code == "ERR_LIP_NOT_INSTALLED" {
				return code
			}
			log.Printf("InstallLIPPackage: failed to query LeviLamina state for %s in %s: %v", normalizedIdentifier, targetName, llQueryErr)
		} else if llState.Installed && !llState.ExplicitInstalled {
			pinnedLLVersion := strings.TrimSpace(llState.InstalledVersion)
			if pinnedLLVersion != "" {
				installPkgs = append(installPkgs, fmt.Sprintf("%s@%s", leviLaminaClientPackageRefBase, pinnedLLVersion))
				hasPinnedLL = true
			}
		}
	}

	state, queryErr := lip.GetPackageInstallStateViaDaemon(ctx, targetDir, packageRefBase)
	if queryErr != nil {
		if code := lip.ErrorCode(queryErr); code == "ERR_LIP_NOT_INSTALLED" {
			return code
		}
		log.Printf("InstallLIPPackage: failed to query package state for %s in %s: %v", normalizedIdentifier, targetName, queryErr)
	}

	shouldUpdate := state.ExplicitInstalled
	if shouldUpdate {
		if err := lip.UpdatePackagesViaDaemon(ctx, targetDir, installPkgs); err != nil {
			if code := lip.ErrorCode(err); code == "ERR_LIP_NOT_INSTALLED" {
				return code
			}
			log.Printf("InstallLIPPackage: lipd update failed for %s: %v", pkg, err)
			return "ERR_LIP_PACKAGE_INSTALL_FAILED"
		}
		return ""
	}

	if err := lip.InstallPackagesViaDaemon(ctx, targetDir, installPkgs); err != nil {
		if code := lip.ErrorCode(err); code == "ERR_LIP_NOT_INSTALLED" {
			return code
		}
		// If the pinned LeviLamina package is already installed and blocks install,
		// retry with target package only.
		if hasPinnedLL && isLipInstallAlreadyInstalledErrorForPackage(err, leviLaminaClientPackageRefBase) {
			retryErr := lip.InstallPackagesViaDaemon(ctx, targetDir, targetOnlyPkgs)
			if retryErr == nil {
				return ""
			}
			err = retryErr
		}

		// Any "already explicitly installed" conflict (including LL dependency)
		// should fallback to update target package only.
		if isLipInstallAlreadyInstalledError(err) {
			if updateErr := lip.UpdatePackagesViaDaemon(ctx, targetDir, targetOnlyPkgs); updateErr != nil {
				if code := lip.ErrorCode(updateErr); code == "ERR_LIP_NOT_INSTALLED" {
					return code
				}
				log.Printf("InstallLIPPackage: fallback update failed for %s: %v", pkg, updateErr)
				return "ERR_LIP_PACKAGE_INSTALL_FAILED"
			}
			return ""
		}

		log.Printf("InstallLIPPackage: lipd install failed for %s: %v", pkg, err)
		return "ERR_LIP_PACKAGE_INSTALL_FAILED"
	}

	return ""
}

func UninstallLIPPackage(ctx context.Context, targetName string, identifier string) string {
	if !lip.IsInstalled() {
		return "ERR_LIP_NOT_INSTALLED"
	}

	_, packageRefBase, ok := buildLIPPackageRefBase(identifier)
	if !ok {
		return "ERR_LIP_PACKAGE_INVALID_IDENTIFIER"
	}

	targetDir, errCode := resolveLIPTargetDir(targetName)
	if errCode != "" {
		return errCode
	}

	state, queryErr := lip.GetPackageInstallStateViaDaemon(ctx, targetDir, packageRefBase)
	if queryErr != nil {
		if code := lip.ErrorCode(queryErr); code == "ERR_LIP_NOT_INSTALLED" {
			return code
		}
		log.Printf("UninstallLIPPackage: failed to query package state for %s in %s: %v", packageRefBase, targetName, queryErr)
	} else if state.Installed && !state.ExplicitInstalled {
		return "ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS"
	}

	if err := lip.UninstallPackagesViaDaemon(ctx, targetDir, []string{packageRefBase}, false); err != nil {
		if code := lip.ErrorCode(err); code == "ERR_LIP_NOT_INSTALLED" {
			return code
		}
		log.Printf("UninstallLIPPackage: lipd uninstall failed for %s: %v", packageRefBase, err)
		return "ERR_LIP_PACKAGE_UNINSTALL_FAILED"
	}

	postState, postQueryErr := lip.GetPackageInstallStateViaDaemon(ctx, targetDir, packageRefBase)
	if postQueryErr != nil {
		if code := lip.ErrorCode(postQueryErr); code == "ERR_LIP_NOT_INSTALLED" {
			return code
		}
		log.Printf("UninstallLIPPackage: post-uninstall query failed for %s in %s: %v", packageRefBase, targetName, postQueryErr)
		return ""
	}

	if postState.Installed && !postState.ExplicitInstalled {
		return "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY"
	}
	if postState.Installed && postState.ExplicitInstalled {
		log.Printf("UninstallLIPPackage: package %s remains explicitly installed in %s after uninstall", packageRefBase, targetName)
		return "ERR_LIP_PACKAGE_UNINSTALL_FAILED"
	}

	return ""
}

func GetLIPPackageInstallState(ctx context.Context, targetName string, identifier string) types.LIPPackageInstallState {
	state := types.LIPPackageInstallState{
		Identifier: normalizeLIPPackageIdentifier(identifier),
	}
	if !lip.IsInstalled() {
		state.Error = "ERR_LIP_NOT_INSTALLED"
		return state
	}

	normalizedIdentifier, packageRefBase, ok := buildLIPPackageRefBase(identifier)
	if !ok {
		state.Error = "ERR_LIP_PACKAGE_INVALID_IDENTIFIER"
		return state
	}

	state.Identifier = normalizedIdentifier
	state.PackageRef = packageRefBase

	targetDir, errCode := resolveLIPTargetDir(targetName)
	if errCode != "" {
		state.Error = errCode
		return state
	}

	daemonState, err := lip.GetPackageInstallStateViaDaemon(ctx, targetDir, packageRefBase)
	if err != nil {
		log.Printf("GetLIPPackageInstallState: lipd query failed for %s in %s: %v", normalizedIdentifier, targetName, err)
		if code := lip.ErrorCode(err); code == "ERR_LIP_NOT_INSTALLED" {
			state.Error = code
			return state
		}
		state.Error = "ERR_LIP_PACKAGE_QUERY_FAILED"
		return state
	}

	state.Installed = daemonState.Installed
	state.ExplicitInstalled = daemonState.ExplicitInstalled
	state.InstalledVersion = daemonState.InstalledVersion
	return state
}
