//go:build windows

// SPDX-License-Identifier: GPL-3.0-only
package nativeinstall

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"os"
)

func ensureDevice() {
	var cached ownDevice
	if e := loadProtected(cachePath(), &cached); e == nil {
		validateDevice(cached)
		return
	} else if !os.IsNotExist(e) {
		failCode("ERR_AUTH_DEVICE_CACHE", "device cache cannot be decrypted; preserve it for recovery")
	}
	checkCanceled()
	random := make([]byte, 32)
	_, e := rand.Read(random)
	must(e)
	defer clear(random)
	member := "02" + hex.EncodeToString(random[:7])
	password := base64.RawURLEncoding.EncodeToString(random[7:25])
	info, e := deviceInfo()
	must(e)
	body := "<?xml version=\"1.0\"?><DeviceAddRequest><ClientInfo name=\"IDCRL\" version=\"1.0\"><BinaryVersion>55</BinaryVersion></ClientInfo><Authentication><Membername>" + member + "</Membername><Password>" + password + "</Password></Authentication>" + info + "</DeviceAddRequest>"
	req, e := http.NewRequestWithContext(active.ctx, "POST", "https://login.live.com/ppsecure/deviceaddcredential.srf", bytes.NewBufferString(body))
	must(e)
	req.Header.Set("Content-Type", "application/soap+xml")
	req.Header.Set("User-Agent", "MSAWindows/55 (OS 10.0.26100.0.0 ge_release; IDK 10.0.26100.5074 ge_release; Cfg 16.000.29325.00; Test 0)")
	res, e := httpClient().Do(req)
	must(e)
	defer res.Body.Close()
	data := boundedBody(res.Body, 1<<20)
	defer clear(data)
	// Explicit XML decoding keeps credentials and license bodies out of diagnostics.
	type response struct {
		Success bool   `xml:"Success,attr"`
		PUID    string `xml:"puid"`
		License struct {
			Block string `xml:"SPLicenseBlock"`
		}
	}
	var parsed response
	must(xml.Unmarshal(data, &parsed))
	if res.StatusCode != http.StatusOK || !parsed.Success || parsed.License.Block == "" {
		failCode("ERR_AUTH_DEVICE_PROVISION", "device provisioning did not return a license")
	}
	state := ownDevice{Member: member, Password: password, PUID: parsed.PUID, License: parsed.License.Block}
	validateDevice(state)
	blocks := parseSP(decode64(state.License))
	key := deviceKey(blocks[1])
	clear(key)
	secret := clep(blocks[0x12d], 544)
	rsaKey(secret)
	clear(secret)
	must(saveProtected(cachePath(), state))
}
