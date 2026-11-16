// SPDX-License-Identifier: GPL-3.0-only
// Package nativeinstall installs complete segmented MSIXVC packages using
// the shared development key or a Windows Microsoft account and an application-owned device
// and online Store licenses for retail packages.
// It never reads system or other applications' private credential stores.
package nativeinstall

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

type Options struct {
	CacheDir           string
	Market             string
	RequireFullLicense bool
	Prepare            func(string) error
	Progress           func(Progress)
	Diagnostic         func(map[string]any)
}
type Progress struct {
	File                                   string
	Current, Total, FileCurrent, FileTotal int64
}
type Report struct {
	ContentID   string
	KeyID       string
	LicenseType string
	OutputDir   string
	FileCount   int
	Bytes       int64
}
type Error struct {
	Code, Reason string
	cause        error
}

func (e *Error) Error() string { return e.Code + ": " + e.Reason }
func (e *Error) Unwrap() error { return e.cause }

// Parsing and COM helpers unwind only this package's typed failures. Other
// panics are programmer errors and are deliberately not converted to success.
func fail(v any)                   { panic(&Error{Code: "ERR_NATIVE_MSIXVC", Reason: fmt.Sprint(v)}) }
func failCode(code, reason string) { panic(&Error{Code: code, Reason: reason}) }
func must(e error) {
	if e != nil {
		panic(&Error{Code: "ERR_NATIVE_MSIXVC", Reason: "native installation operation failed", cause: e})
	}
}

type installation struct {
	ctx     context.Context
	options Options
	source  *os.File
	out     string
	report  Report
	account *licenseAccount
}

var active *installation

// Sensitive device and account values remain within one serialized operation.
var installGate = make(chan struct{}, 1)

func Install(ctx context.Context, sourcePath, outputPath string, options Options) (report Report, err error) {
	if ctx == nil {
		return report, errors.New("nil context")
	}
	select {
	case installGate <- struct{}{}:
	case <-ctx.Done():
		return report, ctx.Err()
	}
	defer func() { <-installGate }()
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(*Error); ok {
				err = e
			} else {
				panic(v)
			}
		}
	}()
	active = &installation{ctx: ctx, options: options}
	defer clearOperation()
	checkCanceled()
	if strconv.IntSize != 64 {
		failCode("ERR_MSIXVC_UNSUPPORTED_FORMAT", "native MSIXVC currently requires a 64-bit host")
	}
	if options.Market == "" {
		active.options.Market = "US"
	}
	market := active.options.Market
	if len(market) != 2 || market[0] < 'A' || market[0] > 'Z' || market[1] < 'A' || market[1] > 'Z' {
		fail("invalid market code")
	}
	out, e := filepath.Abs(outputPath)
	must(e)
	if outputPath == "" || filepath.Base(out) == "." || filepath.Dir(out) == out {
		fail("invalid output directory")
	}
	if _, e = os.Lstat(out); !os.IsNotExist(e) {
		failCode("ERR_TARGET_EXISTS", "output directory already exists or cannot be inspected")
	}
	must(os.MkdirAll(filepath.Dir(out), 0700))
	active.out = out
	in, e := filepath.Abs(sourcePath)
	must(e)
	f, e := openSourceFile(in)
	must(e)
	defer f.Close()
	active.source = f
	active.report.ContentID, active.report.KeyID = readIdentifiers(f)
	if key := developmentContentKey(active.report.KeyID); key != nil {
		contentKey = key
		active.report.LicenseType = "Development"
		emit(map[string]any{"license_type": active.report.LicenseType})
	} else {
		if active.options.CacheDir == "" {
			active.options.CacheDir = apppath.AccountsDir()
		}
		secureCache(active.options.CacheDir)
		lock := lockDeviceCache(active.options.CacheDir)
		defer closeCacheLock(lock)
		if err := RestoreSharedAccount(ctx, active.options.CacheDir); err != nil {
			panic(err)
		}
		beginLicenseAccount(ctx, "")
		if !restoreContentKey() {
			acquireUserTicket(ctx, xbox.GetStoreTicket)
			ticket := deviceTicket()
			license(ticket, active.report.ContentID)
		}
	}
	checkCanceled()
	fullExtract(in, contentKey, active.report.KeyID)
	return active.report, nil
}

func developmentContentKey(keyID string) []byte {
	if !strings.EqualFold(keyID, "33ec8436-5a0e-4f0d-b1ce-3f29c3955039") {
		return nil
	}
	return []byte{
		0x21, 0x75, 0x87, 0xb8, 0xe3, 0x19, 0x45, 0x9c,
		0xba, 0x2e, 0xf2, 0x6f, 0x8d, 0xe6, 0x8e, 0xa8,
		0x9a, 0xb6, 0xdc, 0x0f, 0xbc, 0x11, 0x42, 0xd0,
		0x9f, 0x44, 0x98, 0xb0, 0xbe, 0xe2, 0x24, 0x96,
	}
}

func checkCanceled() {
	if active != nil {
		if e := active.ctx.Err(); e != nil {
			panic(&Error{Code: "ERR_CANCELED", Reason: "installation canceled", cause: e})
		}
		if active.account != nil && active.account.epoch != selectionEpoch.Load() {
			accountAuthorizationFailed(xbox.ErrAccountChanged)
		}
	}
}
func emit(m map[string]any) {
	if active == nil {
		return
	}
	// Keep diagnostics deliberately allowlisted: no raw server error messages,
	// response bodies, account references, tokens, or secret material.
	safe := map[string]any{}
	for _, key := range []string{"response_status", "provider_error_code", "interop_available", "msa_provider_found", "device_binding_matches", "key_wrap_integrity_verified", "rst_http_status", "rst_fault", "device_reprovisioned", "rst_response_signature_verified", "rst_body_decrypted", "own_device_wrapping_key_validated", "license_type", "online_content_id", "online_key_id", "hash_tree_validated", "metadata_pages_validated", "manifest_files", "verified_final_region_padding_bytes", "extraction_complete", "files", "bytes", "output_directory"} {
		if v, ok := m[key]; ok {
			safe[key] = v
		}
	}
	if len(safe) > 0 && active.options.Diagnostic != nil {
		active.options.Diagnostic(safe)
	}
	if current, ok := m["extract_bytes"].(int64); ok && active.options.Progress != nil {
		total, _ := m["extract_total"].(int64)
		file, _ := m["file"].(string)
		fileCurrent, _ := m["file_current"].(int64)
		fileTotal, _ := m["file_total"].(int64)
		active.options.Progress(Progress{File: file, Current: current, Total: total, FileCurrent: fileCurrent, FileTotal: fileTotal})
	}
}
func readIdentifiers(f *os.File) (string, string) {
	stat, e := f.Stat()
	must(e)
	if stat.IsDir() || stat.Size() < 12288 {
		failCode("ERR_MSIXVC_UNSUPPORTED_FORMAT", "input is not a complete MSIXVC")
	}
	h := readRange(f, 0, 4096)
	if string(h[512:520]) != "msft-xvd" || binary.LittleEndian.Uint32(h[0x20c:]) != 3 || binary.LittleEndian.Uint32(h[0x208:]) != 65 || binary.LittleEndian.Uint32(h[0x280:]) != 0 {
		failCode("ERR_MSIXVC_UNSUPPORTED_FORMAT", "unsupported XVD layout")
	}
	drive := binary.LittleEndian.Uint64(h[0x218:])
	user := uint64(binary.LittleEndian.Uint32(h[0x28c:]))
	xvc := uint64(binary.LittleEndian.Uint32(h[0x290:]))
	if drive > 64<<30 || user > 32<<20 || xvc < 0xda8 || xvc > 32<<20 || binary.LittleEndian.Uint32(h[0x294:]) != 0 {
		failCode("ERR_MSIXVC_UNSUPPORTED_FORMAT", "unsupported metadata layout")
	}
	hashed := pages(drive) + pages(user) + pages(xvc)
	total := uint64(0)
	for n := hashed; n > 1; {
		n = (n + 169) / 170
		total += n
	}
	off := uint64(12288) + pages(uint64(binary.LittleEndian.Uint32(h[0x288:])))*4096 + uint64(h[0x470])*4096 + total*4096 + pages(user)*4096
	if off > uint64(stat.Size()) || xvc > uint64(stat.Size())-off {
		fail("truncated XVC metadata")
	}
	x := readRange(f, int64(off), 0xda8)
	if binary.LittleEndian.Uint16(x[0xd1e:]) != 1 {
		failCode("ERR_MSIXVC_UNSUPPORTED_FORMAT", "exactly one content key is currently supported")
	}
	// The licensing service's contentId is VDUID, not XvcInfo.ContentID.
	return formatGUID(h[0x220:0x230]), formatGUID(x[16:32])
}

func cachePath() string { return filepath.Join(active.options.CacheDir, "device.dpapi") }
func validateDevice(d ownDevice) {
	if len(d.Member) < 3 || len(d.Member) > 64 || len(d.Password) < 16 || len(d.Password) > 128 || len(d.PUID) != 16 || len(d.License) == 0 || len(d.License) > 256<<10 {
		fail("invalid application device cache")
	}
	if strings.ContainsAny(d.Member+d.Password, "<>&\"'") {
		fail("invalid device credential characters")
	}
}
