package uwp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Public key bytes and replacement behavior verified against KeyPatcher:
// https://github.com/ambiennt/KeyPatcher/blob/8290a0e99f706f0f34d1730d93b03a5961ba2855/src/main.cpp
// The scanner and persistence below are implemented locally using Go file I/O.
const (
	legacyAuthPublicKey  = "MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE8ELkixyLcwlZryUQcu1TvPOmI2B7vX83ndnWRUaXm74wFfa5f/lwQNTfrLVHa2PmenpGI6JhIMUJaWZrjmMj90NoKNFSNBuKdm8rYiXsfaz3K36x/1U26HpG0ZxK/V1V"
	currentAuthPublicKey = "MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAECRXueJeTDqNRRgJi/vlRufByu/2G0i2Ebt6YMar5QX/R0DIIyrJMcUpruK4QveTfJSTp3Shlq4Gk34cD/4GUWwkv0DVuzeuB+tXija7HBxii03NHDbPAD0AKnLr2wdAp"
	authKeyMarkerName    = ".levilauncher-xbox-auth-key-v1"
	authKeyScanSize      = 1 << 20
)

var authKeyMu sync.Mutex

// ensureLegacyAuthKey checks the actual keys once per installed UWP instance.
// Version numbers are not used: some releases contain both public keys. The small
// sidecar survives version.json updates and folder renames, without hashing or
// opening the executable on subsequent launches. Reinstalling into a fresh
// directory performs the migration again. Callers must ensure the game is idle.
func ensureLegacyAuthKey(ctx context.Context, dir string, manifest Manifest) error {
	authKeyMu.Lock()
	defer authKeyMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	marker := filepath.Join(dir, authKeyMarkerName)
	state, err := os.ReadFile(marker)
	if err == nil {
		switch string(state) {
		case "patched\n", "already-current\n", "not-applicable\n":
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read Xbox authentication key marker: %w", err)
	}
	exe := filepath.Join(dir, manifest.Applications[0].Executable)
	result, err := patchLegacyAuthKey(ctx, exe)
	if err != nil {
		return fmt.Errorf("update Xbox authentication key: %w", err)
	}
	// Publish the marker only after successful scanning, writing, sync and close.
	// A failed marker write is safe to retry: the next scan recognizes the new key.
	if err := os.WriteFile(marker, []byte(result+"\n"), 0644); err != nil {
		return fmt.Errorf("write Xbox authentication key marker: %w", err)
	}
	log.Printf("Xbox authentication key for %s: %s", exe, result)
	return nil
}

func scanLegacyAuthKey(ctx context.Context, file io.ReaderAt) ([]int64, bool, error) {
	buffer := make([]byte, authKeyScanSize)
	var offsets []int64
	var hasCurrent bool
	for base := int64(0); ; {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		n, err := file.ReadAt(buffer, base)
		if err != nil && err != io.EOF {
			return nil, false, err
		}
		chunk := buffer[:n]
		hasCurrent = hasCurrent || bytes.Contains(chunk, []byte(currentAuthPublicKey))
		for start := 0; start < len(chunk); {
			index := bytes.Index(chunk[start:], []byte(legacyAuthPublicKey))
			if index < 0 {
				break
			}
			index += start
			offsets = append(offsets, base+int64(index))
			start = index + len(legacyAuthPublicKey)
		}
		if err == io.EOF {
			return offsets, hasCurrent, nil
		}
		// Retain enough overlap to find keys straddling a buffer boundary.
		base += int64(n - len(legacyAuthPublicKey) + 1)
	}
}

// Patch only full, exact key matches in place; preserve file length, ACLs and
// all other bytes. No executable backup or replacement file is created.
func patchLegacyAuthKey(ctx context.Context, exe string) (result string, err error) {
	if len(legacyAuthPublicKey) != len(currentAuthPublicKey) {
		return "", fmt.Errorf("authentication key lengths differ")
	}
	file, err := os.OpenFile(exe, os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	offsets, hasCurrent, err := scanLegacyAuthKey(ctx, file)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// A binary that already contains the current key must remain unchanged,
	// including transitional releases that contain both the old and new keys.
	if hasCurrent {
		return "already-current", nil
	}
	if len(offsets) == 0 {
		// Some historical packages predate this key/authentication scheme.
		// Record the completed scan without claiming the executable was patched.
		return "not-applicable", nil
	}
	for _, offset := range offsets {
		if _, err := file.WriteAt([]byte(currentAuthPublicKey), offset); err != nil {
			return "", err
		}
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	verify := make([]byte, len(currentAuthPublicKey))
	for _, offset := range offsets {
		if _, err := file.ReadAt(verify, offset); err != nil {
			return "", err
		}
		if string(verify) != currentAuthPublicKey {
			return "", fmt.Errorf("authentication key verification failed at %#x", offset)
		}
	}
	return "patched", nil
}
