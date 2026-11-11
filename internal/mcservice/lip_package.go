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
	lipref "github.com/liteldev/LeviLauncher/internal/lipcore/ref"
	"github.com/liteldev/LeviLauncher/internal/lipindex"
	"github.com/liteldev/LeviLauncher/internal/types"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

var lipIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// leviLaminaPackageID is LeviLamina's package id.
const leviLaminaPackageID = "github.com/LiteLDev/LeviLamina"

// lipClientVariant is the variant that conventionally holds the client
// build of a package (the label "client" of a tooth.json, which lip's
// migration keeps as the variant name). It is what a request without a
// variant asks for when the catalogue cannot say which variant has a client
// entry (see chooseLIPVariant), falling back to the variant "" when lip
// finds no package for it. Installed packages are matched by package id,
// whatever their variant.
const lipClientVariant = "client"

const lipPackageQueryTimeout = 10 * time.Second

// Package operations and queries; replaced in tests.
var (
	lipListPackageStates = lip.ListPackageStates
	lipInstallPackages   = lip.InstallPackages
	lipUpdatePackages    = lip.UpdatePackages
	lipUninstallPackages = lip.UninstallPackages
	lipResolveTargetDir  = resolveLIPTargetDir
	// lipFetchDetail fetches a package's catalogue detail (levimc.org/api/v4/mods).
	lipFetchDetail = func(ctx context.Context, packageID string) (*lipindex.Detail, error) {
		return lipindex.NewClient().FetchDetail(ctx, packageID)
	}
)

type lipPackageIdentifierParts struct {
	base    string
	variant string
	// named is set when the identifier names a variant: "#<variant>", or
	// "#" alone for the variant "".
	named bool
}

// lipPackageRequest is what the launcher asks lip for a frontend identifier
// ("owner/repo", "github.com/owner/repo", optionally with "#variant"). The
// identity encoding is lip's: "<id>#" (or "<id>") is the variant "",
// "<id>#<v>" the variant v. An identifier without "#" names no variant: an
// install picks one with chooseLIPVariant (rule A2 over the catalogue).
type lipPackageRequest struct {
	// identifier is the normalized frontend identifier: "owner/repo", with
	// "#variant" when it names a variant other than "".
	identifier string
	// packageID is "github.com/owner/repo".
	packageID string
	// variant is the variant named, "" for the default variant or none.
	variant string
	// named is set when the identifier names a variant ("#" included).
	named bool
	// requestRef is the source of a new install of a named variant: the
	// package id, with "#variant" for a variant other than "". For a
	// request without a variant it is the bare package id until an install
	// chooses the variant.
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
	named := false
	if idx := strings.Index(normalized, "#"); idx >= 0 {
		named = true
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
		return lipPackageIdentifierParts{base: strings.Join(parts, "/"), variant: variant, named: named}
	}
	if len(parts) == 3 && strings.EqualFold(parts[0], "github.com") {
		return lipPackageIdentifierParts{base: fmt.Sprintf("%s/%s", parts[1], parts[2]), variant: variant, named: named}
	}

	return lipPackageIdentifierParts{}
}

// buildLIPPackageRequest maps a frontend identifier to the lip package it
// names: "#<variant>" names that variant, "#" alone the variant "", and an
// identifier without "#" no variant (an install chooses one).
func buildLIPPackageRequest(identifier string) (lipPackageRequest, bool) {
	parts := canonicalizeLIPPackageIdentifier(identifier)
	if !lipIdentifierPattern.MatchString(parts.base) {
		return lipPackageRequest{}, false
	}
	req := lipPackageRequest{
		identifier: parts.base,
		packageID:  "github.com/" + parts.base,
		variant:    parts.variant,
		named:      parts.named,
	}
	req.requestRef = req.packageID
	if parts.variant != "" {
		req.identifier += "#" + parts.variant
		req.requestRef += "#" + parts.variant
	}
	return req, true
}

// chooseLIPVariant is the variant a request without one installs: rule A2
// of the catalogue (the variant "client", else "", else the first by name,
// among those with an entry for the client on this platform) over the
// package's catalogue detail, for version (or the newest version that has
// such a variant). ok is false when the catalogue cannot say (it is
// unreachable or does not list the package); a package listed without such
// a variant gets "" (lip then reports what is missing).
func chooseLIPVariant(ctx context.Context, packageID, version string) (variant string, ok bool) {
	d, err := lipFetchDetail(ctx, packageID)
	if err != nil {
		log.Printf("chooseLIPVariant: no catalogue detail for %s: %v", packageID, err)
		return "", false
	}
	if v, found := lipindex.ChooseVariant(d, version, lipindex.TargetSide, lipindex.TargetPlatform); found {
		return v, true
	}
	log.Printf("chooseLIPVariant: %s %s has no variant for %s on %s", packageID, version, lipindex.TargetSide, lipindex.TargetPlatform)
	return "", true
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
// installed package (so an installed package is changed in place, whatever
// its variant), otherwise requestRef.
func lipOperationRef(states map[string]lip.PackageInstallState, packageID, variant, requestRef string) string {
	if state, ok := lipInstalledFor(states, packageID, variant); ok && strings.TrimSpace(state.Identity) != "" {
		return state.Identity
	}
	return requestRef
}

func withLIPVersion(packageRef, version string) string {
	return fmt.Sprintf("%s@%s", packageRef, version)
}

// lipRoleRequest is the package an operation requests (ref, without a
// version) and the one it requests instead (fallback) when the target
// release turns out to keep its client build elsewhere: the identity
// follows the target release (rule 5 of LeviLauncher's compatibility with
// tooth.json packages). A release read from a tooth.json keeps its client
// build in the variant "client" ("client_<y>"); one read from a lip.toml,
// which has no client variants, in "" ("<y>"). lip
// (internal/lipcore/engine/compat_legacy_variants.go) replaces the
// installed package of the other identity in place, and tells why a
// request for a client variant found nothing.
type lipRoleRequest struct {
	ref, fallback string
	// onToothJSON: fall back also when the target is a tooth.json release
	// without the client label (its "" is then the only build); otherwise
	// only when it is a lip.toml release.
	onToothJSON bool
}

// lipRoleCanonical is the variant a release read from a lip.toml has for a
// client variant ("client" -> "", "client_<y>" -> "<y>"), as lip's
// compatibility layer reads it; ok is false for other variants.
func lipRoleCanonical(variant string) (string, bool) {
	if variant == lipClientVariant {
		return "", true
	}
	if rest, ok := strings.CutPrefix(variant, lipClientVariant+"_"); ok && lipref.IsValidVariant(rest) {
		return rest, true
	}
	return "", false
}

// lipRoleRequestFor is the request for packageRef (an identity, without a
// version): a client variant falls back to its lip.toml name; an installed
// identity without one first asks for its client variant, in case the
// target is a tooth.json release with that label, and falls back to itself.
func lipRoleRequestFor(packageRef string, installed bool) lipRoleRequest {
	id, variant, _ := strings.Cut(packageRef, "#")
	if canonical, ok := lipRoleCanonical(variant); ok {
		return lipRoleRequest{ref: packageRef, fallback: lipref.Identity(id, canonical)}
	}
	if installed {
		role := lipClientVariant
		if variant != "" {
			role += "_" + variant
		}
		if lipref.IsValidVariant(role) {
			return lipRoleRequest{ref: lipref.Identity(id, role), fallback: packageRef, onToothJSON: true}
		}
	}
	return lipRoleRequest{ref: packageRef}
}

// fallsBack reports whether a failed request for pkg (q.ref at a version)
// is retried with q.fallback.
func (q lipRoleRequest) fallsBack(err error, pkg string) bool {
	if err == nil || q.fallback == "" || !isLipNoPackageFoundFor(err, pkg) {
		return false
	}
	return lip.IsClientVariantNotInLipToml(err) || q.onToothJSON && lip.IsClientVariantNotInToothJSON(err)
}

// isLipRoleIdentity reports whether an identity names a client variant.
func isLipRoleIdentity(identity string) bool {
	_, variant, _ := strings.Cut(identity, "#")
	_, ok := lipRoleCanonical(variant)
	return ok
}

// isLipNoPackageFoundFor reports whether err is lip's failure to find a
// package for the package argument arg.
func isLipNoPackageFoundFor(err error, arg string) bool {
	return lip.IsNoPackageFound(err) &&
		strings.Contains(strings.ToLower(lip.ErrorMessage(err)), "'"+strings.ToLower(arg)+"'")
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
	// The installed package this request changes in place: any installed
	// variant of the package for a request without a variant, else only the
	// variant named.
	state, installed := lipPackageStateFor(states, req.packageID)
	if installed && req.named && !strings.EqualFold(state.Variant, req.variant) {
		state, installed = lip.PackageInstallState{}, false
	}

	var roleReq lipRoleRequest
	switch {
	case installed && strings.TrimSpace(state.Identity) != "":
		// A named variant is kept, unless it is a client variant.
		roleReq = lipRoleRequestFor(state.Identity, !req.named)
	case !req.named:
		// No variant named: the catalogue says which one has a client entry
		// for this platform. When it cannot, ask for the conventional
		// client variant and fall back to the variant "" when the release
		// has none (also a tooth.json release without the label).
		if variant, ok := chooseLIPVariant(ctx, req.packageID, normalizedVersion); ok {
			roleReq = lipRoleRequestFor(lipref.Identity(req.packageID, variant), false)
		} else {
			roleReq = lipRoleRequestFor(req.packageID+"#"+lipClientVariant, false)
			roleReq.onToothJSON = true
		}
	default:
		roleReq = lipRoleRequestFor(req.requestRef, false)
	}

	pkg := withLIPVersion(roleReq.ref, normalizedVersion)
	var pin, pinIdentity string

	// Pin existing LeviLamina version to avoid dependency solver switching LL
	// or fetching a large set of LL candidates from wide ranges (e.g. 1.9.*).
	if lip.PackageKey(req.packageID) != lip.PackageKey(leviLaminaPackageID) {
		if llState, ok := lipPackageStateFor(states, leviLaminaPackageID); ok && !llState.ExplicitInstalled {
			pinnedLLVersion := strings.TrimSpace(llState.InstalledVersion)
			if pinnedLLVersion != "" {
				pin, pinIdentity = withLIPVersion(llState.Identity, pinnedLLVersion), llState.Identity
			}
		}
	}
	withPin := func(pkg string) []string {
		if pin == "" {
			return []string{pkg}
		}
		return []string{pkg, pin}
	}
	// run runs op for pkg with the pin, then for the fallback of the
	// identity (lipRoleRequest) when the release keeps its client build
	// under the other one; and without the pin when it fails and the pin
	// is a client variant ("LeviLamina#client", which the package may need
	// replaced by the release read from a lip.toml).
	run := func(op func(context.Context, string, []string) error, verb string) error {
		err := op(ctx, targetDir, withPin(pkg))
		if roleReq.fallsBack(err, pkg) {
			fallback := withLIPVersion(roleReq.fallback, normalizedVersion)
			log.Printf("InstallLIPPackage: no package for %s, trying %s", pkg, fallback)
			pkg = fallback
			err = op(ctx, targetDir, withPin(pkg))
		}
		if err != nil && pin != "" && isLipRoleIdentity(pinIdentity) && !lip.IsAlreadyExplicitlyInstalled(err) {
			log.Printf("InstallLIPPackage: lip %s failed with %s pinned, trying without: %s", verb, pin, lip.ErrorMessage(err))
			err = op(ctx, targetDir, []string{pkg})
		}
		return err
	}

	if state.ExplicitInstalled {
		if err := run(lipUpdatePackages, "update"); err != nil {
			log.Printf("InstallLIPPackage: lip update failed for %s: %s", pkg, lip.ErrorMessage(err))
			return lip.ErrorResult("ERR_LIP_PACKAGE_INSTALL_FAILED", err)
		}
		return ""
	}

	err := run(lipInstallPackages, "install")
	if err != nil {
		targetOnlyPkgs := []string{pkg}
		// If the pinned LeviLamina package is already installed and blocks install,
		// retry with target package only.
		if pin != "" && isLipAlreadyExplicitlyInstalledFor(err, leviLaminaPackageID) {
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
				return lip.ErrorResult("ERR_LIP_PACKAGE_INSTALL_FAILED", updateErr)
			}
			return ""
		}

		log.Printf("InstallLIPPackage: lip install failed for %s: %s", pkg, lip.ErrorMessage(err))
		return lip.ErrorResult("ERR_LIP_PACKAGE_INSTALL_FAILED", err)
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
		return lip.ErrorResult("ERR_LIP_PACKAGE_UNINSTALL_FAILED", err)
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
