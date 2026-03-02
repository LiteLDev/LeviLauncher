package lip

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventLipInstallStatus   = "lip_install_status"
	EventLipInstallProgress = "lip_install_progress"
	EventLipInstallDone     = "lip_install_done"
	EventLipInstallError    = "lip_install_error"
)

//go:embed lipd.exe
var embeddedLipd []byte

type Status struct {
	Path        string `json:"path"`
	Installed   bool   `json:"installed"`
	UpToDate    bool   `json:"upToDate"`
	LocalSHA    string `json:"localSHA256"`
	EmbeddedSHA string `json:"embeddedSHA256"`
	CanCompare  bool   `json:"canCompare"`
	Error       string `json:"error"`
}

func appExeName() string {
	exeName := "levilauncher.exe"
	if exe, err := os.Executable(); err == nil {
		if b := strings.TrimSpace(filepath.Base(exe)); b != "" {
			exeName = strings.ToLower(b)
		}
	}
	return exeName
}

func binDir() string {
	return filepath.Join(apppath.AppData(), appExeName(), "bin")
}

func LipExePath() string {
	return filepath.Join(binDir(), "lipd.exe")
}

func IsInstalled() bool {
	return utils.FileExists(LipExePath())
}

func GetVersion() string {
	if !IsInstalled() {
		return ""
	}
	cmd := exec.Command(LipExePath(), "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func GetLatestVersion() (string, error) {
	// Compatibility method: no network lookup anymore.
	return strings.TrimSpace(GetVersion()), nil
}

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return strings.ToLower(hex.EncodeToString(h.Sum(nil))), nil
}

func bytesSHA256Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

func extractErrCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "ERR_LIP_INSTALL_FAILED"
	}
	if idx := strings.Index(msg, ":"); idx > 0 {
		head := strings.TrimSpace(msg[:idx])
		if strings.HasPrefix(head, "ERR_") {
			return head
		}
	}
	if strings.HasPrefix(msg, "ERR_") {
		return msg
	}
	return "ERR_LIP_INSTALL_FAILED"
}

func emit(event string, data any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(event, data)
}

func EnsureLatestWithError(ctx context.Context) error {
	_ = ctx

	if len(embeddedLipd) == 0 {
		return fmt.Errorf("ERR_LIP_EMBEDDED_MISSING")
	}

	target := LipExePath()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("ERR_LIP_CREATE_TARGET_DIR: %w", err)
	}

	embeddedSHA := bytesSHA256Hex(embeddedLipd)
	localSHA, err := fileSHA256Hex(target)
	if err == nil && strings.EqualFold(localSHA, embeddedSHA) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ERR_LIP_READ_LOCAL_HASH: %w", err)
	}

	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, embeddedLipd, 0o755); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ERR_LIP_WRITE_FILE: %w", err)
	}
	_ = os.Remove(target)
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ERR_LIP_REPLACE_FILE: %w", err)
	}

	verifySHA, err := fileSHA256Hex(target)
	if err != nil {
		return fmt.Errorf("ERR_LIP_READ_LOCAL_HASH: %w", err)
	}
	if !strings.EqualFold(verifySHA, embeddedSHA) {
		return fmt.Errorf("ERR_LIP_VERIFY_HASH")
	}

	return nil
}

func EnsureLatest(ctx context.Context) {
	if err := EnsureLatestWithError(ctx); err != nil {
		log.Printf("lip: ensure latest failed: %v", err)
	}
}

func CheckStatus() Status {
	out := Status{
		Path:        LipExePath(),
		EmbeddedSHA: bytesSHA256Hex(embeddedLipd),
	}
	if out.EmbeddedSHA != "" {
		out.CanCompare = true
	}

	localSHA, err := fileSHA256Hex(out.Path)
	if err == nil {
		out.Installed = true
		out.LocalSHA = strings.ToLower(localSHA)
	} else if !os.IsNotExist(err) {
		out.Error = fmt.Sprintf("local sha256: %v", err)
	}

	if out.CanCompare && out.Installed {
		out.UpToDate = strings.EqualFold(out.LocalSHA, out.EmbeddedSHA)
	}

	return out
}

func Install() string {
	emit(EventLipInstallStatus, "checking")
	emit(EventLipInstallProgress, map[string]interface{}{
		"percentage": 5.0,
		"current":    0,
		"total":      0,
	})
	st := CheckStatus()

	if !st.Installed || !st.UpToDate {
		emit(EventLipInstallStatus, "preparing")
		emit(EventLipInstallProgress, map[string]interface{}{
			"percentage": 20.0,
			"current":    0,
			"total":      0,
		})
		emit(EventLipInstallStatus, "writing")
		emit(EventLipInstallProgress, map[string]interface{}{
			"percentage": 60.0,
			"current":    0,
			"total":      0,
		})
	}

	if err := EnsureLatestWithError(context.Background()); err != nil {
		code := extractErrCode(err)
		emit(EventLipInstallError, code)
		return code
	}

	emit(EventLipInstallStatus, "verifying")
	emit(EventLipInstallProgress, map[string]interface{}{
		"percentage": 85.0,
		"current":    0,
		"total":      0,
	})
	verifyStatus := CheckStatus()
	if !verifyStatus.Installed || (verifyStatus.CanCompare && !verifyStatus.UpToDate) {
		code := "ERR_LIP_VERIFY_HASH"
		emit(EventLipInstallError, code)
		return code
	}

	emit(EventLipInstallStatus, "done")
	emit(EventLipInstallProgress, map[string]interface{}{
		"percentage": 100.0,
		"current":    0,
		"total":      0,
	})
	emit(EventLipInstallDone, GetVersion())
	return ""
}
