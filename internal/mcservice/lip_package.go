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

// leviLaminaPackageID is LeviLamina's package id. The launcher requests the
// bare repository: on the client platform lip maps the "client" label of a
// tooth.json to the variant "".
const leviLaminaPackageID = "github.com/LiteLDev/LeviLamina"

// lipClientVariant is the variant lip v0.x used for client packages. Requests
// for it are made without a variant; installed packages recorded with it (in
// workspaces migrated from tooth_lock.json) are matched by package id.
const lipClientVariant = "client"

const lipPackageQueryTimeout = 10 * time.Second

// Package operations and queries; replaced in tests.
var (
	lipListPackageStates = lip.ListPackageStates
	lipInstallPackages   = lip.InstallPackages
	lipUpdatePackages    = lip.UpdatePackages
	lipUninstallPackages = lip.UninstallPackages
	lipResolveTargetDir  = resolveLIPTargetDir
)

type lipPackageIdentifierParts struct {
	base    string
	variant string
}

// lipPackageRequest is what the launcher asks lip for a frontend identifier
// ("owner/repo", "github.com/owner/repo", optionally with "#variant").
type lipPackageRequest struct {
	// identifier is the normalized frontend identifier: "owner/repo", with
	// "#variant" for a variant other than client.
	identifier string
	// packageID is "github.com/owner/repo".
	packageID string
	// variant is the requested variant other than client, or "".
	variant string
	// requestRef is the source of a new install: the package id, with
	// "#variant" for a variant other than client.
	requestRef string
}

type lipPackageInstallStateRequest struct {
	Identifier        string
	IdentifierKey     string
	pkg               lipPackageRequest
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

	variant := ""
	if idx := strings.Index(normalized, "#"); idx >= 0 {
		variant = strings.TrimSpace(normalized[idx+1:])
		normalized = strings.TrimSpace(normalized[:idx])
	}
	// A version suffix ("@1.2.3") is not part of the identifier.
	if idx := strings.Index(variant, "@"); idx >= 0 {
		variant = strings.TrimSpace(variant[:idx])
	} else if idx := strings.Index(normalized, "@"); idx >= 0 {
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
		return lipPackageIdentifierParts{base: strings.Join(parts, "/"), variant: variant}
	}
	if len(parts) == 3 && strings.EqualFold(parts[0], "github.com") {
		return lipPackageIdentifierParts{base: fmt.Sprintf("%s/%s", parts[1], parts[2]), variant: variant}
	}

	return lipPackageIdentifierParts{}
}

// buildLIPPackageRequest maps a frontend identifier to the lip package it
// names. The variant "client" (or none) requests the bare repository.
func buildLIPPackageRequest(identifier string) (lipPackageRequest, bool) {
	parts := canonicalizeLIPPackageIdentifier(identifier)
	if !lipIdentifierPattern.MatchString(parts.base) {
		return lipPackageRequest{}, false
	}
	req := lipPackageRequest{
		identifier: parts.base,
		packageID:  "github.com/" + parts.base,
	}
	req.requestRef = req.packageID
	if parts.variant != "" && !strings.EqualFold(parts.variant, lipClientVariant) {
		req.variant = parts.variant
		req.identifier += "#" + parts.variant
		req.requestRef += "#" + parts.variant
	}
	return req, true
}

// lipPackageStateFor returns the installed package with req's package id,
// whatever its variant.
func lipPackageStateFor(states map[string]lip.PackageInstallState, packageID string) (lip.PackageInstallState, bool) {
	state, ok := states[lip.PackageKey(packageID)]
	if !ok || !state.Installed {
		return lip.PackageInstallState{}, false
	}
	return state, true
}

// lipInstalledFor returns the installed package that a request for packageID
// with variant changes: the installed variant of the package when the
// request names no variant (or client), otherwise only an installed package
// of that variant.
func lipInstalledFor(states map[string]lip.PackageInstallState, packageID, variant string) (lip.PackageInstallState, bool) {
	state, ok := lipPackageStateFor(states, packageID)
	if !ok || (variant != "" && !strings.EqualFold(state.Variant, variant)) {
		return lip.PackageInstallState{}, false
	}
	return state, true
}

// lipOperationRef is the package argument for an operation on packageID with
// variant: the identity the workspace records when lipInstalledFor finds an
// installed package (so a "#client" package migrated from tooth_lock.json is
// changed in place), otherwise requestRef.
func lipOperationRef(states map[string]lip.PackageInstallState, packageID, variant, requestRef string) string {
	if state, ok := lipInstalledFor(states, packageID, variant); ok && strings.TrimSpace(state.Identity) != "" {
		return state.Identity
	}
	return requestRef
}

func withLIPVersion(packageRef, version string) string {
	return fmt.Sprintf("%s@%s", packageRef, version)
}

func resolveLIPTargetDir(targetName string) (string, string) {
	if GetVersionMeta(targetName).PackageType == "uwp" {
		return "", "ERR_UWP_UNSUPPORTED_FEATURE"
	}
	vdir, err := apppath.InstancesDir()
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

// queryLIPPackageStates lists the installed packages of targetDir keyed by
// package id, with the package query timeout.
func queryLIPPackageStates(ctx context.Context, targetDir string) (map[string]lip.PackageInstallState, error) {
	queryCtx, cancel := context.WithTimeout(ctx, lipPackageQueryTimeout)
	defer cancel()
	return lipListPackageStates(queryCtx, targetDir)
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
		preparedBaseState: types.LIPPackageInstallState{
			Identifier: normalizedInput,
		},
	}

	pkg, ok := buildLIPPackageRequest(normalizedInput)
	if !ok {
		req.preparedBaseState.Error = "ERR_LIP_PACKAGE_INVALID_IDENTIFIER"
		return req
	}

	req.pkg = pkg
	req.preparedBaseState.Identifier = pkg.identifier
	req.preparedBaseState.PackageRef = pkg.requestRef
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

	statesByID, err := queryLIPPackageStates(ctx, targetDir)
	var errorCode string
	if err != nil {
		errorCode = "ERR_LIP_PACKAGE_QUERY_FAILED"
	}

	for _, req := range requests {
		state := req.preparedBaseState
		switch {
		case state.Error != "":
		case errorCode != "":
			state.Error = errorCode
		default:
			if matched, ok := lipPackageStateFor(statesByID, req.pkg.packageID); ok {
				state.PackageRef = matched.Identity
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

// isLipAlreadyExplicitlyInstalledFor reports whether err is lip's refusal to
// install packageRef's package because it is already explicitly installed.
func isLipAlreadyExplicitlyInstalledFor(err error, packageRef string) bool {
	if !lip.IsAlreadyExplicitlyInstalled(err) {
		return false
	}
	spec, ok := lip.ParsePackageSpec(packageRef)
	if !ok {
		return true
	}
	// lip quotes the identity ('<id>' or '<id>#<variant>').
	msg := strings.ToLower(lip.ErrorMessage(err))
	id := strings.ToLower(spec.ID)
	return strings.Contains(msg, "'"+id+"'") || strings.Contains(msg, "'"+id+"#")
}

func InstallLIPPackage(ctx context.Context, targetName string, identifier string, version string) string {
	req, ok := buildLIPPackageRequest(identifier)
	if !ok {
		return "ERR_LIP_PACKAGE_INVALID_IDENTIFIER"
	}

	normalizedVersion := strings.TrimSpace(version)
	if normalizedVersion == "" {
		return "ERR_LIP_PACKAGE_VERSION_REQUIRED"
	}

	targetDir, errCode := lipResolveTargetDir(targetName)
	if errCode != "" {
		return errCode
	}

	states, stateQueryErr := queryLIPPackageStates(ctx, targetDir)
	if stateQueryErr != nil {
		log.Printf("InstallLIPPackage: failed to query install states for %s in %s: %v", req.identifier, targetName, stateQueryErr)
	}
	state, _ := lipInstalledFor(states, req.packageID, req.variant)

	pkg := withLIPVersion(lipOperationRef(states, req.packageID, req.variant, req.requestRef), normalizedVersion)
	targetOnlyPkgs := []string{pkg}
	installPkgs := []string{pkg}
	hasPinnedLL := false

	// Pin existing LeviLamina version to avoid dependency solver switching LL
	// or fetching a large set of LL candidates from wide ranges (e.g. 1.9.*).
	if lip.PackageKey(req.packageID) != lip.PackageKey(leviLaminaPackageID) {
		if llState, ok := lipPackageStateFor(states, leviLaminaPackageID); ok && !llState.ExplicitInstalled {
			pinnedLLVersion := strings.TrimSpace(llState.InstalledVersion)
			if pinnedLLVersion != "" {
				installPkgs = append(installPkgs, withLIPVersion(llState.Identity, pinnedLLVersion))
				hasPinnedLL = true
			}
		}
	}

	if state.ExplicitInstalled {
		if err := lipUpdatePackages(ctx, targetDir, installPkgs); err != nil {
			log.Printf("InstallLIPPackage: lip update failed for %s: %s", pkg, lip.ErrorMessage(err))
			return "ERR_LIP_PACKAGE_INSTALL_FAILED"
		}
		return ""
	}

	if err := lipInstallPackages(ctx, targetDir, installPkgs); err != nil {
		// If the pinned LeviLamina package is already installed and blocks install,
		// retry with target package only.
		if hasPinnedLL && isLipAlreadyExplicitlyInstalledFor(err, leviLaminaPackageID) {
			retryErr := lipInstallPackages(ctx, targetDir, targetOnlyPkgs)
			if retryErr == nil {
				return ""
			}
			err = retryErr
		}

		// Any "already explicitly installed" conflict (including LL dependency)
		// should fallback to update target package only.
		if lip.IsAlreadyExplicitlyInstalled(err) {
			if updateErr := lipUpdatePackages(ctx, targetDir, targetOnlyPkgs); updateErr != nil {
				log.Printf("InstallLIPPackage: fallback update failed for %s: %s", pkg, lip.ErrorMessage(updateErr))
				return "ERR_LIP_PACKAGE_INSTALL_FAILED"
			}
			return ""
		}

		log.Printf("InstallLIPPackage: lip install failed for %s: %s", pkg, lip.ErrorMessage(err))
		return "ERR_LIP_PACKAGE_INSTALL_FAILED"
	}

	return ""
}

// uninstallLIPPackageByID uninstalls the installed package with packageID
// (by its recorded identity), refusing one that is only a dependency, and
// reports ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY when it stays installed as a
// dependency.
func uninstallLIPPackageByID(ctx context.Context, caller, targetName, targetDir, packageID, requestRef string) string {
	states, preQueryErr := queryLIPPackageStates(ctx, targetDir)
	if preQueryErr != nil {
		log.Printf("%s: failed to query package state for %s in %s: %v", caller, packageID, targetName, preQueryErr)
	}
	state, installed := lipPackageStateFor(states, packageID)
	if installed && !state.ExplicitInstalled {
		return "ERR_LIP_PACKAGE_REQUIRED_BY_DEPENDENTS"
	}
	packageRef := lipOperationRef(states, packageID, "", requestRef)

	if err := lipUninstallPackages(ctx, targetDir, []string{packageRef}, false); err != nil {
		log.Printf("%s: lip uninstall failed for %s: %s", caller, packageRef, lip.ErrorMessage(err))
		return "ERR_LIP_PACKAGE_UNINSTALL_FAILED"
	}

	postStates, postQueryErr := queryLIPPackageStates(ctx, targetDir)
	if postQueryErr != nil {
		log.Printf("%s: post-uninstall query failed for %s in %s: %v", caller, packageRef, targetName, postQueryErr)
		return ""
	}
	postState, stillInstalled := lipPackageStateFor(postStates, packageID)
	if stillInstalled && !postState.ExplicitInstalled {
		return "ERR_LIP_PACKAGE_DEMOTED_TO_DEPENDENCY"
	}
	if stillInstalled && postState.ExplicitInstalled {
		log.Printf("%s: package %s remains explicitly installed in %s after uninstall", caller, packageRef, targetName)
		return "ERR_LIP_PACKAGE_UNINSTALL_FAILED"
	}

	return ""
}

func UninstallLIPPackage(ctx context.Context, targetName string, identifier string) string {
	req, ok := buildLIPPackageRequest(identifier)
	if !ok {
		return "ERR_LIP_PACKAGE_INVALID_IDENTIFIER"
	}

	targetDir, errCode := lipResolveTargetDir(targetName)
	if errCode != "" {
		return errCode
	}

	return uninstallLIPPackageByID(ctx, "UninstallLIPPackage", targetName, targetDir, req.packageID, req.requestRef)
}

func GetLIPPackageInstallStates(ctx context.Context, targetName string, identifiers []string) []types.LIPPackageInstallStateEntry {
	requests := make([]lipPackageInstallStateRequest, 0, len(identifiers))
	for _, identifier := range identifiers {
		requests = append(requests, buildLIPPackageInstallStateRequest(identifier, identifier))
	}

	entries, err := queryLIPPackageInstallStates(ctx, targetName, requests)
	if err != nil {
		log.Printf("GetLIPPackageInstallStates: lip query failed in %s: %v", targetName, err)
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
