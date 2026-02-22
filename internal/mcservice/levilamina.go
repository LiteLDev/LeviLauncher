package mcservice

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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

func InstallLeviLamina(ctx context.Context, mcVersion string, targetName string) string {
	if !lip.IsInstalled() {
		return "ERR_LIP_NOT_INSTALLED"
	}

	db, err := FetchLeviLaminaVersionDB()
	if err != nil {
		log.Println("InstallLeviLamina: Fetch DB failed:", err)
		return "ERR_FETCH_LL_DB"
	}

	parts := strings.Split(mcVersion, ".")
	if len(parts) < 3 {
		return "ERR_INVALID_VERSION_FORMAT"
	}
	key := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])

	versions, ok := db[key]
	if !ok || len(versions) == 0 {
		return "ERR_LL_NOT_SUPPORTED"
	}

	llVersion := versions[len(versions)-1]

	vdir, err := apppath.VersionsDir()
	if err != nil {
		return "ERR_ACCESS_VERSIONS_DIR"
	}
	targetDir := filepath.Join(vdir, targetName)
	if !utils.DirExists(targetDir) {
		return "ERR_TARGET_NOT_FOUND"
	}

	log.Printf("Installing LeviLamina %s for %s using LIP", llVersion, targetName)

	lipExe := lip.LipExePath()
	pkg := fmt.Sprintf("github.com/LiteLDev/LeviLamina#client@%s", llVersion)

	cmd := exec.CommandContext(ctx, lipExe, "install", pkg, "-y")
	cmd.Dir = targetDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("InstallLeviLamina: lip install failed: %v, output: %s", err, string(out))
		return "ERR_LIP_INSTALL_FAILED"
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

	lipExe := lip.LipExePath()
	pkgs := []string{"github.com/LiteLDev/LeviLamina#client", "github.com/LiteLDev/bedrock-runtime-data"}

	args := append([]string{"uninstall"}, pkgs...)
	args = append(args, "-y")

	cmd := exec.CommandContext(ctx, lipExe, args...)
	cmd.Dir = targetDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("UninstallLeviLamina: lip uninstall failed: %v, output: %s", err, string(out))
		return "ERR_LIP_UNINSTALL_FAILED"
	}

	return ""
}
