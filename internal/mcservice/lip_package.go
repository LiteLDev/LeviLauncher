package mcservice

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

var lipIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

const leviLaminaClientPackageRefBase = "github.com/LiteLDev/LeviLamina#client"
const lipPackageQueryTimeout = 10 * time.Second

var lipIsInstalled = lip.IsInstalled
var lipListPackageStatesViaDaemon = lip.ListPackageStatesViaDaemon
var lipResolveTargetDir = resolveLIPTargetDir

type lipPackageIdentifierParts struct {
	base             string
	variant          string
	hasVariantMarker bool
}

type lipPackageInstallStateRequest struct {
	Identifier        string
	IdentifierKey     string
	lookupKey         string
	normalizedID      string
	packageRefBase    string
	preparedBaseState types.LIPPackageInstallState
}

func normalizeLIPPackageIdentifier(identifier string) string {
	return strings.TrimSpace(identifier)
}

func canonicalizeLIPPackageIdentifier(identifier string) lipPackageIdentifierParts {
	normalized := normalizeLIPPackageIdentifier(identifier)
	if normalized == "" {
		return lipPackageIdentifierParts{}
	}

	hasVariantMarker := false
	variant := ""
	if idx := strings.Index(normalized, "#"); idx >= 0 {
		hasVariantMarker = true
		variant = strings.TrimSpace(normalized[idx+1:])
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
		return lipPackageIdentifierParts{}
	}

	parts := strings.FieldsFunc(normalized, func(r rune) bool { return r == '/' })
	if len(parts) == 2 {
		return lipPackageIdentifierParts{
			base:             strings.Join(parts, "/"),
			variant:          variant,
			hasVariantMarker: hasVariantMarker,
		}
	}
	if len(parts) == 3 && strings.Contains(parts[0], ".") {
		return lipPackageIdentifierParts{
			base:             fmt.Sprintf("%s/%s", parts[1], parts[2]),
			variant:          variant,
			hasVariantMarker: hasVariantMarker,
		}
	}

	return lipPackageIdentifierParts{}
}

func buildLIPPackageRefBase(identifier string) (string, string, bool) {
	parts := canonicalizeLIPPackageIdentifier(identifier)
	if !lipIdentifierPattern.MatchString(parts.base) {
		return "", "", false
	}

	normalizedIdentifier := parts.base
	packageRefBase := fmt.Sprintf("github.com/%s#client", parts.base)
	if parts.variant != "" {
		normalizedIdentifier = fmt.Sprintf("%s#%s", normalizedIdentifier, parts.variant)
		packageRefBase = fmt.Sprintf("github.com/%s#%s", parts.base, parts.variant)
	} else if parts.hasVariantMarker {
		normalizedIdentifier = normalizedIdentifier + "#"
		packageRefBase = fmt.Sprintf("github.com/%s", parts.base)
	}

	return normalizedIdentifier, packageRefBase, true
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

func buildLIPPackageInstallStateRequest(identifier string, identifierKey string) lipPackageInstallStateRequest {
	normalizedIdentifierKey := strings.TrimSpace(identifierKey)
	normalizedInput := normalizeLIPPackageIdentifier(identifier)
	if normalizedIdentifierKey == "" {
		normalizedIdentifierKey = normalizedInput
	}

	req := lipPackageInstallStateRequest{
		Identifier:    normalizedInput,
		IdentifierKey: normalizedIdentifierKey,
		lookupKey:     strings.ToLower(normalizedIdentifierKey),
		preparedBaseState: types.LIPPackageInstallState{
			Identifier: normalizedInput,
		},
	}

	normalizedIdentifier, packageRefBase, ok := buildLIPPackageRefBase(normalizedInput)
	if !ok {
		req.preparedBaseState.Error = "ERR_LIP_PACKAGE_INVALID_IDENTIFIER"
		return req
	}

	req.normalizedID = normalizedIdentifier
	req.packageRefBase = packageRefBase
	req.preparedBaseState.Identifier = normalizedIdentifier
	req.preparedBaseState.PackageRef = packageRefBase
	return req
}

func queryLIPPackageInstallStates(
	ctx context.Context,
	targetName string,
	requests []lipPackageInstallStateRequest,
) ([]types.LIPPackageInstallStateEntry, error) {
	entries := make([]types.LIPPackageInstallStateEntry, 0, len(requests))
	if len(requests) == 0 {
		return entries, nil
	}

	if !lipIsInstalled() {
		for _, req := range requests {
			state := req.preparedBaseState
			state.Error = "ERR_LIP_NOT_INSTALLED"
			entries = append(entries, types.LIPPackageInstallStateEntry{
				IdentifierKey: req.IdentifierKey,
				State:         state,
			})
		}
		return entries, nil
	}

	targetDir, errCode := lipResolveTargetDir(targetName)
	if errCode != "" {
		for _, req := range requests {
			state := req.preparedBaseState
			state.Error = errCode
			entries = append(entries, types.LIPPackageInstallStateEntry{
				IdentifierKey: req.IdentifierKey,
				State:         state,
			})
		}
		return entries, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, lipPackageQueryTimeout)
	defer cancel()

	stateByPackageRef, err := lipListPackageStatesViaDaemon(queryCtx, targetDir)
	var errorCode string
	if err != nil {
		errorCode = "ERR_LIP_PACKAGE_QUERY_FAILED"
		if code := lip.ErrorCode(err); code == "ERR_LIP_NOT_INSTALLED" {
			errorCode = code
		}
	}

	for _, req := range requests {
		state := req.preparedBaseState
		switch {
		case state.Error != "":
		case errorCode != "":
			state.Error = errorCode
		default:
			lookupKey := strings.ToLower(req.packageRefBase)
			if matched, ok := stateByPackageRef[lookupKey]; ok {
				state.Installed = matched.Installed
				state.ExplicitInstalled = matched.ExplicitInstalled
				state.InstalledVersion = matched.InstalledVersion
			}
		}

		entries = append(entries, types.LIPPackageInstallStateEntry{
			IdentifierKey: req.IdentifierKey,
			State:         state,
		})
	}

	return entries, err
}

func getInstallStateEntryMap(entries []types.LIPPackageInstallStateEntry) map[string]types.LIPPackageInstallState {
	stateByLookupKey := make(map[string]types.LIPPackageInstallState, len(entries))
	for _, entry := range entries {
		lookupKey := strings.ToLower(strings.TrimSpace(entry.IdentifierKey))
		if lookupKey == "" {
			continue
		}
		if _, exists := stateByLookupKey[lookupKey]; exists {
			continue
		}
		stateByLookupKey[lookupKey] = entry.State
	}
	return stateByLookupKey
}

func InstallLIPPackage(ctx context.Context, targetName string, identifier string, version string) string {
	if !lipIsInstalled() {
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
	stateRequests := []lipPackageInstallStateRequest{
		buildLIPPackageInstallStateRequest(leviLaminaClientPackageRefBase, leviLaminaClientPackageRefBase),
		buildLIPPackageInstallStateRequest(identifier, identifier),
	}
	stateEntries, stateQueryErr := queryLIPPackageInstallStates(ctx, targetName, stateRequests)
	stateByLookupKey := getInstallStateEntryMap(stateEntries)
	if stateQueryErr != nil {
		log.Printf("InstallLIPPackage: failed to query install states for %s in %s: %v", normalizedIdentifier, targetName, stateQueryErr)
	}

	// Pin existing LeviLamina version to avoid dependency solver switching LL
	// or fetching a large set of LL candidates from wide ranges (e.g. 1.9.*).
	if !strings.EqualFold(packageRefBase, leviLaminaClientPackageRefBase) {
		llState := stateByLookupKey[strings.ToLower(strings.TrimSpace(leviLaminaClientPackageRefBase))]
		if llState.Error == "ERR_LIP_NOT_INSTALLED" {
			return llState.Error
		}
		if llState.Error != "" {
			log.Printf("InstallLIPPackage: failed to query LeviLamina state for %s in %s: %s", normalizedIdentifier, targetName, llState.Error)
		} else if llState.Installed && !llState.ExplicitInstalled {
			pinnedLLVersion := strings.TrimSpace(llState.InstalledVersion)
			if pinnedLLVersion != "" {
				installPkgs = append(installPkgs, fmt.Sprintf("%s@%s", leviLaminaClientPackageRefBase, pinnedLLVersion))
				hasPinnedLL = true
			}
		}
	}

	state := stateByLookupKey[strings.ToLower(strings.TrimSpace(identifier))]
	if state.Error == "ERR_LIP_NOT_INSTALLED" {
		return state.Error
	}
	if state.Error != "" {
		log.Printf("InstallLIPPackage: failed to query package state for %s in %s: %s", normalizedIdentifier, targetName, state.Error)
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
	if !lipIsInstalled() {
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

	preEntries, preQueryErr := queryLIPPackageInstallStates(ctx, targetName, []lipPackageInstallStateRequest{
		buildLIPPackageInstallStateRequest(identifier, identifier),
	})
	if preQueryErr != nil {
		log.Printf("UninstallLIPPackage: failed to query package state for %s in %s: %v", packageRefBase, targetName, preQueryErr)
	}
	preStates := getInstallStateEntryMap(preEntries)
	state := preStates[strings.ToLower(strings.TrimSpace(identifier))]
	if state.Error == "ERR_LIP_NOT_INSTALLED" {
		return state.Error
	}
	if state.Error != "" {
		log.Printf("UninstallLIPPackage: failed to query package state for %s in %s: %s", packageRefBase, targetName, state.Error)
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

	postEntries, postQueryErr := queryLIPPackageInstallStates(ctx, targetName, []lipPackageInstallStateRequest{
		buildLIPPackageInstallStateRequest(identifier, identifier),
	})
	if postQueryErr != nil {
		log.Printf("UninstallLIPPackage: post-uninstall query failed for %s in %s: %v", packageRefBase, targetName, postQueryErr)
	}
	postStates := getInstallStateEntryMap(postEntries)
	postState := postStates[strings.ToLower(strings.TrimSpace(identifier))]
	if postState.Error == "ERR_LIP_NOT_INSTALLED" {
		return postState.Error
	}
	if postState.Error != "" {
		log.Printf("UninstallLIPPackage: post-uninstall query failed for %s in %s: %s", packageRefBase, targetName, postState.Error)
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

func GetLIPPackageInstallStates(ctx context.Context, targetName string, identifiers []string) []types.LIPPackageInstallStateEntry {
	requests := make([]lipPackageInstallStateRequest, 0, len(identifiers))
	for _, identifier := range identifiers {
		requests = append(requests, buildLIPPackageInstallStateRequest(identifier, identifier))
	}

	entries, err := queryLIPPackageInstallStates(ctx, targetName, requests)
	if err != nil {
		log.Printf("GetLIPPackageInstallStates: lipd query failed in %s: %v", targetName, err)
	}
	return entries
}

func GetLIPPackageInstallState(ctx context.Context, targetName string, identifier string) types.LIPPackageInstallState {
	entries := GetLIPPackageInstallStates(ctx, targetName, []string{identifier})
	if len(entries) == 0 {
		return types.LIPPackageInstallState{
			Identifier: normalizeLIPPackageIdentifier(identifier),
			Error:      "ERR_LIP_PACKAGE_QUERY_FAILED",
		}
	}
	return entries[0].State
}
