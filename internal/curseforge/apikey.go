// Package curseforge resolves the CurseForge API credentials used by the
// launcher's CurseForge client.
package curseforge

import (
	_ "embed"
	"encoding/base64"
	"os"
)

//go:embed apikey.bin
var obfuscatedAPIKey []byte

const apiKeyMask = 0xAF

// APIKey returns the CurseForge API key, preferring the CURSEFORGE_API_KEY
// environment variable over the embedded build-time key.
func APIKey() string {
	if key := os.Getenv("CURSEFORGE_API_KEY"); key != "" {
		return key
	}

	deobfuscated := make([]byte, len(obfuscatedAPIKey))
	for i, b := range obfuscatedAPIKey {
		deobfuscated[i] = b ^ apiKeyMask
	}

	decoded, err := base64.StdEncoding.DecodeString(string(deobfuscated))
	if err != nil {
		return ""
	}
	return string(decoded)
}
