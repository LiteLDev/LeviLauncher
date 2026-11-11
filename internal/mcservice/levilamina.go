package mcservice

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/httpx"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

type LeviLaminaVersionDB struct {
	FormatVersion int                 `json:"format_version"`
	Versions      map[string][]string `json:"versions"`
}

var fetchLeviLaminaVersionDB = FetchLeviLaminaVersionDB

func resolveSupportedLeviLaminaVersions(db map[string][]string, mcVersion string) []string {
	v := strings.TrimSpace(mcVersion)
	if v == "" {
		return nil
	}
	if exact, ok := db[v]; ok && len(exact) > 0 {
		return exact
	}
	return nil
}

func pickPreferredLeviLaminaVersion(versions []string) string {
	sorted := sortLeviLaminaVersions(versions)
	if len(sorted) > 0 {
		return sorted[0]
	}
	return ""
}

func compareLeviLaminaVersion(a string, b string) int {
	parse := func(v string) (*semver.Version, error) {
		return semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	}
	av, aErr := parse(a)
	bv, bErr := parse(b)
	if aErr == nil && bErr == nil {
		return av.Compare(bv)
	}
	if aErr == nil {
		return 1
	}
	if bErr == nil {
		return -1
	}
	return strings.Compare(strings.TrimSpace(a), strings.TrimSpace(b))
}

func sortLeviLaminaVersions(versions []string) []string {
	seen := make(map[string]struct{}, len(versions))
	normalized := make([]string, 0, len(versions))
	for _, version := range versions {
		trimmed := strings.TrimSpace(version)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return compareLeviLaminaVersion(normalized[i], normalized[j]) > 0
	})
	return normalized
}

func findRequestedLeviLaminaVersion(requested string, supported []string) (string, bool) {
	want := strings.TrimSpace(requested)
	if want == "" {
		return "", false
	}
	for _, version := range supported {
		if strings.EqualFold(strings.TrimSpace(version), want) {
			return strings.TrimSpace(version), true
		}
	}
	return "", false
}

func FetchLeviLaminaVersionDB() (map[string][]string, error) {
	urls := []string{
		"https://raw.githubusercontent.com/LiteLDev/levilamina-client-version-db/refs/heads/main/v2/version-db.json",
	}
	if utils.IsMirrorEnabled() {
		urls = append([]string{
			"https://gh-proxy.org/https://github.com/LiteLDev/levilamina-client-version-db/raw/refs/heads/main/v2/version-db.json",
			"https://gh.bibk.top/https://github.com/LiteLDev/levilamina-client-version-db/raw/refs/heads/main/v2/version-db.json",
		}, urls...)
	}

	var lastErr error
	for _, url := range urls {
		client := httpx.NewClient(10 * time.Second)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		httpx.ApplyDefaultHeaders(req)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("FetchLeviLaminaVersionDB error fetching %s: %v", url, err)
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d from %s", resp.StatusCode, url)
			continue
		}

		var db LeviLaminaVersionDB
		err = json.NewDecoder(resp.Body).Decode(&db)
		resp.Body.Close()
		if err != nil {
			log.Printf("FetchLeviLaminaVersionDB decode error from %s: %v", url, err)
			lastErr = err
			continue
		}

		return db.Versions, nil
	}

	return nil, lastErr
}

func InstallLeviLamina(ctx context.Context, mcVersion string, targetName string, llVersion string) string {
	if GetVersionMeta(targetName).PackageType == "uwp" {
		return "ERR_UWP_UNSUPPORTED_FEATURE"
	}

	db, err := fetchLeviLaminaVersionDB()
	if err != nil {
		log.Println("InstallLeviLamina: Fetch DB failed:", err)
		return "ERR_FETCH_LL_DB"
	}

	parts := strings.Split(strings.TrimSpace(mcVersion), ".")
	if len(parts) < 3 {
		return "ERR_INVALID_VERSION_FORMAT"
	}
	versions := sortLeviLaminaVersions(resolveSupportedLeviLaminaVersions(db, mcVersion))
	if len(versions) == 0 {
		return "ERR_LL_NOT_SUPPORTED"
	}
	requested := strings.TrimSpace(llVersion)
	if requested == "" {
		llVersion = pickPreferredLeviLaminaVersion(versions)
	} else {
		matched, ok := findRequestedLeviLaminaVersion(requested, versions)
		if !ok {
			return "ERR_LL_VERSION_UNSUPPORTED"
		}
		llVersion = matched
	}
	if strings.TrimSpace(llVersion) == "" {
		return "ERR_LL_NOT_SUPPORTED"
	}

	vdir, err := apppath.InstancesDir()
	if err != nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	targetDir := filepath.Join(vdir, targetName)
	if !utils.DirExists(targetDir) {
		return "ERR_TARGET_NOT_FOUND"
	}

	// Change an installed LeviLamina in place, whatever variant it was
	// installed with; otherwise request its client variant.
	states, listErr := queryLIPPackageStates(ctx, targetDir)
	if listErr != nil {
		log.Printf("InstallLeviLamina: lip list failed, fallback to install-first strategy: %v", listErr)
	}
	// The identity follows the target release (lipRunSwitching); a tooth.json release
	// without the client label holds only a server build, which is not
	// installed on a client.
	state, installed := lipInstalledFor(states, leviLaminaPackageID, "")
	ref := leviLaminaPackageID + "#" + lipClientVariant
	if installed && strings.TrimSpace(state.Identity) != "" {
		ref = state.Identity
	}
	var pkg string
	run := func(op func(context.Context, string, []string) error) error {
		var err error
		pkg, err = lipRunSwitching(ctx, "InstallLeviLamina", op, targetDir, ref, llVersion, nil, false)
		return err
	}

	if state.ExplicitInstalled {
		log.Printf("Updating LeviLamina %s for %s using LIP", llVersion, targetName)
		if err := run(lipUpdatePackages); err != nil {
			log.Printf("InstallLeviLamina: lip update failed: %s", lip.ErrorMessage(err))
			return "ERR_LIP_INSTALL_FAILED"
		}
		return ""
	}

	log.Printf("Installing LeviLamina %s for %s using LIP", llVersion, targetName)
	err = run(lipInstallPackages)
	if err != nil {
		if !isLipAlreadyExplicitlyInstalledFor(err, leviLaminaPackageID) {
			log.Printf("InstallLeviLamina: lip install failed: %s", lip.ErrorMessage(err))
			return "ERR_LIP_INSTALL_FAILED"
		}
		log.Printf("InstallLeviLamina: package already installed, switching to update: %s", lip.ErrorMessage(err))
		if updateErr := lipUpdatePackages(ctx, targetDir, []string{pkg}); updateErr != nil {
			log.Printf("InstallLeviLamina: lip update failed: %s", lip.ErrorMessage(updateErr))
			return "ERR_LIP_INSTALL_FAILED"
		}
	}

	return ""
}

func UninstallLeviLamina(ctx context.Context, targetName string) string {
	vdir, err := apppath.InstancesDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	targetDir := filepath.Join(vdir, targetName)
	if !utils.DirExists(targetDir) {
		return "ERR_TARGET_NOT_FOUND"
	}

	log.Printf("Uninstalling LeviLamina for %s using LIP", targetName)
	return uninstallLIPPackageByID(ctx, "UninstallLeviLamina", targetName, targetDir, leviLaminaPackageID, leviLaminaPackageID)
}
