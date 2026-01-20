package mcservice

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/corpix/uarand"
	json "github.com/goccy/go-json"
	"github.com/liteldev/LeviLauncher/internal/lip"
	"github.com/liteldev/LeviLauncher/internal/utils"
)

type LeviLaminaVersionDB struct {
	FormatVersion int                 `json:"format_version"`
	Versions      map[string][]string `json:"versions"`
}

func FetchLeviLaminaVersionDB() (map[string][]string, error) {
	const url = "https://github.bibk.top/LiteLDev/levilamina-client-version-db/raw/refs/heads/main/version-db.json"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", uarand.GetRandom())

	resp, err := client.Do(req)
	if err != nil {
		log.Println("FetchLeviLaminaVersionDB error:", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var db LeviLaminaVersionDB
	if err := json.NewDecoder(resp.Body).Decode(&db); err != nil {
		log.Println("FetchLeviLaminaVersionDB decode error:", err)
		return nil, err
	}

	return db.Versions, nil
}

func InstallLeviLamina(ctx context.Context, mcVersion string, targetName string) string {
	if !lip.IsInstalled() {
		return "ERR_LIP_NOT_INSTALLED"
	}

	// 1. Fetch DB
	db, err := FetchLeviLaminaVersionDB()
	if err != nil {
		log.Println("InstallLeviLamina: Fetch DB failed:", err)
		return "ERR_FETCH_LL_DB"
	}

	// 2. Resolve MC Version to Key
	// Logic: 1.21.50.07 -> 1.21.50
	parts := strings.Split(mcVersion, ".")
	if len(parts) < 3 {
		return "ERR_INVALID_VERSION_FORMAT"
	}
	key := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])

	versions, ok := db[key]
	if !ok || len(versions) == 0 {
		return "ERR_LL_NOT_SUPPORTED"
	}

	// 3. Pick latest LL version (last is latest)
	llVersion := versions[len(versions)-1]

	// 4. Install using LIP
	vdir, err := utils.GetVersionsDir()
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

func downloadFile(ctx context.Context, url string, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", uarand.GetRandom())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
