//go:build windows

package xbox

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"os"
	"testing"
	"time"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func updateResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body)), Header: make(http.Header)}
}

func TestDeviceAuthSignsRequest(t *testing.T) {
	original := httpClient
	t.Cleanup(func() { httpClient = original })
	var previousID, previousKey string
	httpClient = &http.Client{Transport: updateTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://device.auth.xboxlive.com/device/authenticate" || req.Method != http.MethodPost {
			t.Fatalf("unexpected device endpoint: %s %s", req.Method, req.URL)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		var request struct {
			RelyingParty, TokenType string
			Properties              struct {
				AuthMethod, Id, DeviceType, Version string
				ProofKey                            struct{ Crv, Alg, Use, Kty, X, Y string }
			}
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatal(err)
		}
		p := request.Properties
		if request.RelyingParty != "http://auth.xboxlive.com" || request.TokenType != "JWT" || p.AuthMethod != "ProofOfPossession" || p.DeviceType != "Win32" || p.Version == "" || p.ProofKey.Crv != "P-256" || p.ProofKey.Alg != "ES256" || p.ProofKey.Use != "sig" || p.ProofKey.Kty != "EC" {
			t.Fatal("invalid device authentication contract")
		}
		if len(p.Id) != 36 || p.Id == previousID || p.ProofKey.X == previousKey {
			t.Fatal("device identity/key reused")
		}
		previousID, previousKey = p.Id, p.ProofKey.X
		x, _ := base64.RawURLEncoding.DecodeString(p.ProofKey.X)
		y, _ := base64.RawURLEncoding.DecodeString(p.ProofKey.Y)
		publicKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		if len(x) != 32 || len(y) != 32 || !publicKey.Curve.IsOnCurve(publicKey.X, publicKey.Y) {
			t.Fatal("invalid proof key")
		}
		signature, err := base64.StdEncoding.DecodeString(req.Header.Get("Signature"))
		if err != nil || len(signature) != 76 {
			t.Fatal("invalid signature envelope")
		}
		policy := binary.BigEndian.Uint32(signature[:4])
		filetime := binary.BigEndian.Uint64(signature[4:12])
		now := time.Now().Unix()
		seconds := int64(filetime/10000000) - 11644473600
		if policy != 1 || filetime%10000000 != 0 || seconds < now-5 || seconds > now+5 {
			t.Fatal("invalid policy or Windows timestamp")
		}
		var signed bytes.Buffer
		_ = binary.Write(&signed, binary.BigEndian, policy)
		signed.WriteByte(0)
		_ = binary.Write(&signed, binary.BigEndian, filetime)
		signed.WriteByte(0)
		signed.WriteString("POST\x00/device/authenticate\x00\x00")
		signed.Write(body)
		signed.WriteByte(0)
		digest := sha256.Sum256(signed.Bytes())
		r := new(big.Int).SetBytes(signature[12:44])
		s := new(big.Int).SetBytes(signature[44:])
		if !ecdsa.Verify(publicKey, digest[:], r, s) {
			t.Fatal("signature does not authenticate actual request bytes")
		}
		return updateResponse(`{"Token":"device-token"}`), nil
	})}
	for i := 0; i < 2; i++ {
		token, err := xblDeviceAuth(context.Background())
		if err != nil || token != "device-token" {
			t.Fatalf("device auth failed: %v", err)
		}
	}
}

func TestPackageUpdateAuthorizationCacheAndAccountSelection(t *testing.T) {
	original := httpClient
	t.Cleanup(func() { httpClient = original; ConfigureAccountSelection("", nil) })
	ConfigureAccountSelection("selected", nil)
	profile := &session{accountID: "selected", xsts: "profile-token", uhs: "hash", userToken: "user-token", exp: time.Now().Add(time.Hour)}
	cached = profile
	calls := 0
	httpClient = &http.Client{Transport: updateTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Host == "device.auth.xboxlive.com" {
			return updateResponse(`{"Token":"device-token"}`), nil
		}
		if req.URL.Host != "xsts.auth.xboxlive.com" {
			t.Fatalf("unexpected auth request: %s", req.URL.Host)
		}
		var body struct {
			RelyingParty string
			Properties   struct {
				DeviceToken, SandboxId string
				UserTokens             []string
			}
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.RelyingParty != "http://update.xboxlive.com" || body.Properties.DeviceToken != "device-token" || body.Properties.SandboxId != "RETAIL" || len(body.Properties.UserTokens) != 1 || body.Properties.UserTokens[0] != "user-token" {
			t.Fatal("package authorization not bound to selected profile and update relying party")
		}
		return updateResponse(`{"Token":"update-token","DisplayClaims":{"xui":[{"uhs":"hash"}]}}`), nil
	})}
	for i := 0; i < 2; i++ {
		header, err := GetPackageUpdateAuthorization(context.Background())
		if err != nil || header != "XBL3.0 x=hash;update-token" {
			t.Fatalf("update authorization failed: %v", err)
		}
	}
	if calls != 2 || cached != profile || cached.xsts != "profile-token" {
		t.Fatal("update auth replaced profile or bypassed token cache")
	}
	profile.update.exp = time.Now().Add(-time.Second)
	if _, err := GetPackageUpdateAuthorization(context.Background()); err != nil || calls != 4 {
		t.Fatal("expired update token not refreshed")
	}
	ConfigureAccountSelection("other", ErrAuthenticationFailed)
	if _, err := GetPackageUpdateAuthorization(context.Background()); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("account restore failure ignored: %v", err)
	}
	ConfigureAccountSelection("", nil)
	if _, err := GetPackageUpdateAuthorization(context.Background()); !errors.Is(err, ErrInteractionRequired) {
		t.Fatalf("signed-out discovery selected a default account: %v", err)
	}
	if calls != 4 {
		t.Fatal("account switch reused stale update authorization")
	}
}

func TestPackageUpdateAuthorizationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GetPackageUpdateAuthorization(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}

func TestDeviceAuthLive(t *testing.T) {
	if os.Getenv("LEVI_XBOX_UPDATE_PROBE") != "1" {
		t.Skip("explicit live probe required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := xblDeviceAuth(ctx); err != nil {
		t.Fatal(err)
	}
}
