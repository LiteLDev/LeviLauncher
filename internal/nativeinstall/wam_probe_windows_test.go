//go:build windows && wamprobe

package nativeinstall

import (
	"context"
	"errors"
	"github.com/liteldev/LeviLauncher/internal/xbox"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Tests the production account and license paths, without extraction or installation.
// Only allowlisted diagnostics are logged; credentials and keys never leave the backend.
func TestWAMStoreLicense(t *testing.T) {
	if os.Getenv("LEVI_WAM_PROBE") != "1" {
		t.Skip("set LEVI_WAM_PROBE=1 for live authentication")
	}
	source := os.Getenv("LEVI_WAM_PROBE_PACKAGE")
	if source == "" {
		t.Fatal("set LEVI_WAM_PROBE_PACKAGE to a local retail MSIXVC")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir()
	if cache := os.Getenv("LEVI_WAM_PROBE_CACHE"); cache != "" {
		names := []string{"wam-account.dpapi"}
		if os.Getenv("LEVI_WAM_PROBE_FRESH_DEVICE") != "1" {
			names = append(names, "device.dpapi")
		}
		for _, name := range names {
			data, e := os.ReadFile(filepath.Join(cache, name))
			if os.IsNotExist(e) && name != "device.dpapi" {
				continue
			}
			if e != nil {
				t.Fatal("probe cache unavailable")
			}
			e = os.WriteFile(filepath.Join(dir, name), data, 0600)
			clear(data)
			if e != nil {
				t.Fatal("cache copy failed")
			}
		}
	}
	if e := RestoreSharedAccount(ctx, dir); e != nil {
		t.Fatal("account preference restoration failed")
	}
	defer xbox.ConfigureAccountSelection("", nil)
	if err := xbox.ResetSession(); err != nil {
		t.Fatal(err)
	}
	stage := "start"
	err := accountOperation(ctx, dir, func() {
		active.options.Market = "US"
		active.options.RequireFullLicense = true
		active.options.Diagnostic = func(d map[string]any) { t.Logf("diagnostic=%v", d) }
		stage = "read_package"
		f, e := os.Open(source)
		must(e)
		defer f.Close()
		active.report.ContentID, active.report.KeyID = readIdentifiers(f)
		if developmentContentKey(active.report.KeyID) != nil {
			fail("probe requires a retail package")
		}
		stage = "device_registration"
		ensureDevice()
		stage = "shared_account"
		acquireUserTicket(ctx, xbox.GetStoreTicket)
		// Repeat acquisition through the cached launcher session and assert identity stability.
		token, reference, e := xbox.GetStoreTicket(ctx)
		must(e)
		if token == "" || reference != userReference {
			fail("shared account identity changed")
		}
		token = ""
		stage = "device_ticket"
		device := ownDeviceTicket()
		stage = "license"
		license(device, active.report.ContentID)
		t.Logf("license_type=%s key_verified=%t", active.report.LicenseType, len(contentKey) == 32)
	})
	if err != nil {
		code := "PROBE_ERROR"
		var failure *Error
		if errors.As(err, &failure) {
			code = failure.Code
		}
		t.Fatalf("stage=%s error_code=%s", stage, code)
	}
	if userTicket != "" || userReference != "" || len(contentKey) != 0 {
		t.Fatal("credential/key cleanup incomplete")
	}
}
