//go:build darwin

package nativeinstall

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/fsx"
	"os"
	"path/filepath"
)

var protectedKey = systemProtectedKey

const protectedHeader = "LEVI-NATIVE-1\x00"

func protectedAccount(path string) string {
	p, _ := filepath.Abs(filepath.Dir(path))
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:])
}
func protectedCipher(path string, create bool) (cipher.AEAD, error) {
	key, err := protectedKey(protectedAccount(path), create)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	if len(key) != 32 {
		return nil, errors.New("invalid system credential key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func loadProtected(path string, out any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < int64(len(protectedHeader)) || info.Size() > 1<<20 {
		return errors.New("invalid protected cache")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	defer clear(b)
	if string(b[:len(protectedHeader)]) != protectedHeader {
		return errors.New("cache belongs to another protection provider; sign in using a separate native account directory")
	}
	a, err := protectedCipher(path, false)
	if err != nil {
		return err
	}
	b = b[len(protectedHeader):]
	if len(b) < a.NonceSize()+a.Overhead() {
		return errors.New("truncated protected cache")
	}
	plain, err := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(filepath.Base(path)))
	if err != nil {
		return fmt.Errorf("protected cache authentication: %w", err)
	}
	defer clear(plain)
	return json.Unmarshal(plain, out)
}
func saveProtected(path string, value any) error {
	plain, err := json.Marshal(value)
	if err != nil {
		return err
	}
	defer clear(plain)
	a, err := protectedCipher(path, true)
	if err != nil {
		return err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	b := append([]byte(protectedHeader), nonce...)
	b = a.Seal(b, nonce, plain, []byte(filepath.Base(path)))
	defer clear(b)
	return fsx.WriteFileAtomic(path, b, 0600)
}
