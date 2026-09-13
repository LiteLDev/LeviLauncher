// SPDX-License-Identifier: GPL-3.0-only
// Store wire contracts: Xodus 0670e25aeb0e0e9f800f8f2f4968ae3b681842a7.
package nativeinstall

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/beevik/etree"
	"io"
	"net/http"
	"strings"
	"time"
)

var userTicket, userReference string
var contentKey []byte

func httpClient() *http.Client {
	return &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func boundedBody(r io.Reader, max int64) []byte {
	b, e := io.ReadAll(io.LimitReader(r, max+1))
	must(e)
	if int64(len(b)) > max {
		fail("response exceeds size limit")
	}
	return b
}

type contentLicenseResponse struct {
	License             *struct{ Keys []struct{ Value string } }
	SatisfactionFailure *struct {
		Code        int64
		Description string
	}
}

func requestContentLicense(device, contentID string) contentLicenseResponse {
	challenge := base64.StdEncoding.EncodeToString([]byte("<?xml version=\"1.0\" encoding=\"utf-8\"?><ClientChallenge xmlns=\"http://schemas.microsoft.com/onestore/security/mkms/LicReq/v1\" Version=\"2\"><LicenseProtocolVersion>5</LicenseProtocolVersion><SigningKeyVersion>1</SigningKeyVersion><ClientVersion>2</ClientVersion></ClientChallenge>"))
	body := map[string]any{
		"clientChallenge": challenge, "concurrencyMode": "Rude", "contentId": contentID,
		"deviceContext":  map[string]string{"hardwareManufacturer": "Public", "hardwareType": "Public", "mobileOperator": "Public"},
		"licenseVersion": 4, "market": active.options.Market, "needKey": true, "keyOnly": true,
		"users": map[string]any{"S-1-5-21-0000000000-0000000000-0000000000-1001": []any{
			map[string]string{"identityType": "Msa", "identityValue": userTicket, "localTicketReference": userReference},
		}},
	}
	payload, e := json.Marshal(body)
	must(e)
	defer clear(payload)
	req, e := http.NewRequestWithContext(active.ctx, "POST", "https://licensing.mp.microsoft.com/v7.0/licenses/content", bytes.NewReader(payload))
	must(e)
	req.Header.Set("Authorization", device)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("From", "XboxLicenseManager")
	req.Header.Set("User-Agent", "XboxLm-PC/Microsoft.GamingServices_32.107.4002.0_x64__8wekyb3d8bbwe")
	res, e := httpClient().Do(req)
	must(e)
	defer res.Body.Close()
	data := boundedBody(res.Body, 2<<20)
	defer clear(data)
	if res.StatusCode != http.StatusOK {
		failCode("ERR_LICENSE_HTTP", "content license HTTP request failed")
	}
	var parsed contentLicenseResponse
	must(json.Unmarshal(data, &parsed))
	return parsed
}

type licensedContentKey struct {
	KeyID, LicenseType string
	Key                []byte
}

func clearLicensedKeys(keys []licensedContentKey) {
	for i := range keys {
		clear(keys[i].Key)
	}
}

func parseContentLicense(parsed contentLicenseResponse, requireFull bool) (keys []licensedContentKey) {
	complete := false
	defer func() {
		if !complete {
			clearLicensedKeys(keys)
		}
	}()
	if parsed.SatisfactionFailure != nil {
		failCode("ERR_LICENSE_NOT_ENTITLED", "account has no entitlement for this package")
	}
	if parsed.License == nil || len(parsed.License.Keys) == 0 || len(parsed.License.Keys) > 16 {
		failCode("ERR_LICENSE_MISSING_KEY", "no usable content license returned")
	}
	for _, record := range parsed.License.Keys {
		b := decode64(record.Value)
		defer clear(b)
		doc := etree.NewDocument()
		must(doc.ReadFromBytes(b))
		clear(b)
		if doc.Root() == nil {
			failCode("ERR_LICENSE_UNSUPPORTED", "empty content license document")
		}
		kind := only(doc.Root(), "LicenseInfo").SelectAttrValue("Type", "")
		if kind != "Full" && kind != "Trial" {
			failCode("ERR_LICENSE_UNSUPPORTED", "unsupported content license type")
		}
		if requireFull && kind != "Full" {
			failCode("ERR_LICENSE_FULL_REQUIRED", "the service returned a trial license")
		}
		blob := decode64(only(doc.Root(), "SPLicenseBlock").Text())
		defer clear(blob)
		keys = append(keys, unpackContentKeys(blob, kind)...)
	}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if seen[key.KeyID] {
			fail("ambiguous duplicate content key")
		}
		seen[key.KeyID] = true
	}
	if len(keys) == 0 || len(keys) > 256 {
		failCode("ERR_LICENSE_MISSING_KEY", "no usable content keys returned")
	}
	complete = true
	return keys
}

func license(device, contentID string) {
	keys := parseContentLicense(requestContentLicense(device, contentID), active.options.RequireFullLicense)
	defer clearLicensedKeys(keys)
	checkCanceled()
	clear(contentKey)
	contentKey = nil
	for _, key := range keys {
		if strings.EqualFold(key.KeyID, active.report.KeyID) {
			contentKey = append([]byte(nil), key.Key...)
			active.report.LicenseType = key.LicenseType
			emit(map[string]any{"online_content_id": contentID, "online_key_id": key.KeyID, "license_type": key.LicenseType, "device_binding_matches": true, "key_wrap_integrity_verified": true})
		}
	}
	if len(contentKey) != 32 {
		failCode("ERR_LICENSE_MISSING_KEY", "license does not contain the package KeyID")
	}
	cacheLicenseKeys(contentID, "", keys)
}
