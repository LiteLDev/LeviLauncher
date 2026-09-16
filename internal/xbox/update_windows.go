//go:build windows

package xbox

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GetPackageUpdateAuthorization authorizes package discovery for the selected
// account. The update token stays in memory, attached to the profile session so
// resetting or switching accounts also discards it. This never opens a picker.
func GetPackageUpdateAuthorization(ctx context.Context) (string, error) {
	if err := lockSession(ctx); err != nil {
		return "", err
	}
	defer func() { <-sessionGate }()
	s, err := ensureSessionLocked(ctx)
	if err != nil {
		return "", err
	}
	if s.update != nil && time.Now().Before(s.update.exp) {
		return s.update.authHeader(), nil
	}
	deviceToken, err := xblDeviceAuth(ctx)
	if err != nil {
		return "", err
	}
	update, err := xblXSTSForParty(ctx, s.userToken, deviceToken, "http://update.xboxlive.com")
	if err != nil {
		return "", err
	}
	s.update = update
	return update.authHeader(), nil
}

func xblDeviceAuth(ctx context.Context) (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	body, err := json.Marshal(map[string]any{
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
		"Properties": map[string]any{
			"AuthMethod": "ProofOfPossession",
			"Id":         fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]),
			"DeviceType": "Win32",
			"Version":    "10.0.19042",
			"ProofKey": map[string]string{
				"crv": "P-256", "alg": "ES256", "use": "sig", "kty": "EC",
				"x": base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32))),
				"y": base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32))),
			},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://device.auth.xboxlive.com/device/authenticate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	signature, err := deviceRequestSignature(key, body, time.Now())
	if err != nil {
		return "", err
	}
	req.Header.Set("Signature", signature)
	req.Header.Set("Content-Type", "application/json")
	var out struct{ Token string }
	if err := doJSON(req, "1", "", &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("ERR_XBL_DEVICE_AUTH_EMPTY")
	}
	return out.Token, nil
}

// Xbox proof-of-possession wire format, following PrismarineJS/prismarine-auth
// b795199dc5fa26059655bb1bc91c7f7f2733b232 (MIT; see THIRD_PARTY_NOTICES).
// Sign this request's body with a fresh key rather than replay a shared device.
func deviceRequestSignature(key *ecdsa.PrivateKey, body []byte, now time.Time) (string, error) {
	header := make([]byte, 12, 76)
	binary.BigEndian.PutUint32(header[:4], 1)
	binary.BigEndian.PutUint64(header[4:], uint64(now.Unix()+11644473600)*10000000)
	hash := sha256.New()
	for _, part := range [][]byte{header[:4], header[4:], []byte("POST"), []byte("/device/authenticate"), nil, body} {
		_, _ = hash.Write(part)
		_, _ = hash.Write([]byte{0})
	}
	r, s, err := ecdsa.Sign(rand.Reader, key, hash.Sum(nil))
	if err != nil {
		return "", err
	}
	header = append(header, r.FillBytes(make([]byte, 32))...)
	header = append(header, s.FillBytes(make([]byte, 32))...)
	return base64.StdEncoding.EncodeToString(header), nil
}
