package uwp

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// The fixture is AppxSignature.p7x of Minecraft for Windows 1.21.114.1 as
// served by Windows Update; its block map hashed to the AXBM below.
const fixtureBlockMapSHA256 = "aba28b4f21f9ef3a89157989d8fd5b752ca6327b688c401c611423afb5969516"

func TestVerifyAppxSignatureAcceptsMicrosoftPackage(t *testing.T) {
	p7x, err := os.ReadFile("testdata/minecraft-1.21.114.1.p7x")
	if err != nil {
		t.Fatal(err)
	}
	digests, signer, err := verifyAppxSignature(p7x)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := hex.EncodeToString(digests["AXBM"]); got != fixtureBlockMapSHA256 {
		t.Fatalf("AXBM = %s", got)
	}
	ct, err := os.ReadFile("testdata/minecraft-1.21.114.1.content_types.xml")
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(ct); hex.EncodeToString(sum[:]) != hex.EncodeToString(digests["AXCT"]) {
		t.Fatal("AXCT does not match [Content_Types].xml")
	}
	t.Logf("signer: %s", signer.Subject)
}

func TestVerifyAppxSignatureRejectsTampering(t *testing.T) {
	p7x, err := os.ReadFile("testdata/minecraft-1.21.114.1.p7x")
	if err != nil {
		t.Fatal(err)
	}
	// Flip one byte inside the signed APPX digest blob.
	tampered := append([]byte(nil), p7x...)
	for i := 0; i+4 < len(tampered); i++ {
		if string(tampered[i:i+4]) == "AXBM" {
			tampered[i+10] ^= 0xFF
			break
		}
	}
	if _, _, err := verifyAppxSignature(tampered); err == nil {
		t.Fatal("tampered digest accepted")
	}
	// Flip one byte of the RSA signature at the end of the signer info.
	sig := append([]byte(nil), p7x...)
	sig[len(sig)/2] ^= 0x01
	if _, _, err := verifyAppxSignature(sig); err == nil {
		t.Fatal("corrupted signature accepted")
	}
	if _, _, err := verifyAppxSignature([]byte("PKCXjunk")); err == nil {
		t.Fatal("junk accepted")
	}
}
