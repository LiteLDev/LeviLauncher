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

func resolveSupportedLeviLaminaVersions(db map[string][]string, mcVersion string) []string {
	v := strings.TrimSpace(mcVersion)
	if v == "" {
		return nil
	}
	if exact, ok := db[v]; ok && len(exact) > 0 {
		return exact
	}
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		return nil
	}
	key := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])
	if byMajorMinorPatch, ok := db[key]; ok && len(byMajorMinorPatch) > 0 {
		return byMajorMinorPatch
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

func isLipInstallAlreadyInstalledError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "already explicitly installed") ||
		(strings.Contains(msg, "cannot install package") && strings.Contains(msg, "already installed"))
}

func FetchLeviLaminaVersionDB() (map[string][]string, error) {
	urls := []string{
		"https://cdn.jsdelivr.net/gh/LiteLDev/levilamina-client-version-db@main/version-db.json",
		"https://fastly.jsdelivr.net/gh/LiteLDev/levilamina-client-version-db@main/version-db.json",
		"https://raw.githubusercontent.com/LiteLDev/levilamina-client-version-db/refs/heads/main/version-db.json",
		"https://github.bibk.top/LiteLDev/levilamina-client-version-db/raw/refs/heads/main/version-db.json",
	}

	var lastErr error
	for _, url := range urls {
		client := &http.Client{Timeout: 10 * time.Second}
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
	if !lip.IsInstalled() {
		return "ERR_LIP_NOT_INSTALLED"
	}

	db, err := FetchLeviLaminaVersionDB()
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

	vdir, err := apppath.VersionsDir()
	if err != nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	targetDir := filepath.Join(vdir, targetName)
	if !utils.DirExists(targetDir) {
		return "ERR_TARGET_NOT_FOUND"
	}

	basePkg := "github.com/LiteLDev/LeviLamina#client"
	pkg := fmt.Sprintf("%s@%s", basePkg, llVersion)
	explicitInstalled, listErr := lip.IsPackageExplicitlyInstalledViaDaemon(ctx, targetDir, basePkg)
	if listErr != nil {
		log.Printf("InstallLeviLamina: lipd list failed, fallback to install-first strategy: %v", listErr)
	}

	if explicitInstalled {
		log.Printf("Updating LeviLamina %s for %s using LIP", llVersion, targetName)
		if err := lip.UpdatePackagesViaDaemon(ctx, targetDir, []string{pkg}); err != nil {
			log.Printf("InstallLeviLamina: lipd update failed: %v", err)
			return "ERR_LIP_INSTALL_FAILED"
		}
		return ""
	}

	log.Printf("Installing LeviLamina %s for %s using LIP", llVersion, targetName)
	if err := lip.InstallPackagesViaDaemon(ctx, targetDir, []string{pkg}); err != nil {
		if !isLipInstallAlreadyInstalledError(err) {
			log.Printf("InstallLeviLamina: lipd install failed: %v", err)
			return "ERR_LIP_INSTALL_FAILED"
		}
		log.Printf("InstallLeviLamina: package already installed, switching to update: %v", err)
		if updateErr := lip.UpdatePackagesViaDaemon(ctx, targetDir, []string{pkg}); updateErr != nil {
			log.Printf("InstallLeviLamina: lipd update failed: %v", updateErr)
			return "ERR_LIP_INSTALL_FAILED"
		}
	}

	return ""
}

func UninstallLeviLamina(ctx context.Context, targetName string) string {
	if !lip.IsInstalled() {
		return "ERR_LIP_NOT_INSTALLED"
	}

	vdir, err := apppath.VersionsDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	targetDir := filepath.Join(vdir, targetName)
	if !utils.DirExists(targetDir) {
		return "ERR_TARGET_NOT_FOUND"
	}

	log.Printf("Uninstalling LeviLamina for %s using LIP", targetName)

	// Uninstall the top-level package only; dependency cleanup is resolved by LIP.
	pkgs := []string{"github.com/LiteLDev/LeviLamina#client"}
	if err := lip.UninstallPackagesViaDaemon(ctx, targetDir, pkgs); err != nil {
		log.Printf("UninstallLeviLamina: lipd uninstall failed: %v", err)
		return "ERR_LIP_UNINSTALL_FAILED"
	}

	return ""
}
