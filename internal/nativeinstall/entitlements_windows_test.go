//go:build windows

package nativeinstall

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestContentLicenseStates(t *testing.T) {
	licenseJSON := func(xml string) string {
		return fmt.Sprintf(`{"license":{"keys":[{"value":%q}]}}`, base64.StdEncoding.EncodeToString([]byte(xml)))
	}
	for _, tc := range []struct{ name, body, want string }{
		{"full", licenseJSON(`<License><LicenseInfo Type="Full"/></License>`), "authorized"},
		{"trial", licenseJSON(`<License><LicenseInfo Type="Trial"/></License>`), "trial"},
		{"denied", `{"satisfactionFailure":{"code":1}}`, "not_entitled"},
		{"empty", `{}`, "error"},
		{"empty-keys", `{"license":{"keys":[]}}`, "error"},
		{"invalid-encoding", `{"license":{"keys":[{"value":"!"}]}}`, "error"},
		{"invalid-xml", licenseJSON(`<License`), "error"},
		{"missing-type", licenseJSON(`<License><LicenseInfo/></License>`), "error"},
		{"unknown-type", licenseJSON(`<License><LicenseInfo Type="Other"/></License>`), "error"},
		{"duplicate-info", licenseJSON(`<License><LicenseInfo Type="Full"/><LicenseInfo Type="Trial"/></License>`), "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var response contentLicenseResponse
			if err := json.Unmarshal([]byte(tc.body), &response); err != nil {
				t.Fatal(err)
			}
			if got := contentLicenseState(response); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func catalogFixture(product, family, content string) string {
	return fmt.Sprintf(`{"Product":{"ProductId":%q,"DisplaySkuAvailabilities":[{"Sku":{"Properties":{"Packages":[{"ContentId":%q,"PackageFormat":"MSIXVC","PackageFamilyName":%q,"Architectures":["x64"]}]}}}]}}`, product, content, family)
}

func TestCatalogRejectsWrongProductAndUnsupportedPackages(t *testing.T) {
	valid := catalogFixture("release", "release-family", "7792d9ce-355a-493c-afbd-768f4a77c3b0")
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"valid", valid, true},
		{"wrong-product", strings.ReplaceAll(valid, `"ProductId":"release"`, `"ProductId":"preview"`), false},
		{"wrong-family", strings.ReplaceAll(valid, "release-family", "preview-family"), false},
		{"trial", strings.ReplaceAll(valid, `"Properties":{`, `"Properties":{"IsTrial":true,`), false},
		{"preorder", strings.ReplaceAll(valid, `"Properties":{`, `"Properties":{"IsPreOrder":true,`), false},
		{"wrong-architecture", strings.ReplaceAll(valid, "x64", "arm64"), false},
		{"wrong-format", strings.ReplaceAll(valid, "MSIXVC", "APPX"), false},
		{"invalid-content", strings.ReplaceAll(valid, "7792d9ce-355a-493c-afbd-768f4a77c3b0", "invalid"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data catalogProduct
			if err := json.Unmarshal([]byte(tc.body), &data); err != nil {
				t.Fatal(err)
			}
			err := caught(func() { selectCatalogContentID(data, "release", "release-family") })
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected catalog result: %v", err)
			}
		})
	}
}

func TestChannelLicenseHTTPFailuresAreNotMissingEntitlements(t *testing.T) {
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original; clearOperation() }()
	active = &installation{ctx: context.Background(), options: Options{Market: "US"}}
	userTicket, userReference = "test-ticket", "test-account"
	for _, status := range []int{401, 403, 429, 500} {
		http.DefaultTransport = licenseTransport(func(req *http.Request) (*http.Response, error) {
			body := catalogFixture("release", "family", "7792d9ce-355a-493c-afbd-768f4a77c3b0")
			code := 200
			if req.URL.Host == "licensing.mp.microsoft.com" {
				code = status
				body = `{"satisfactionFailure":{"code":1}}`
			}
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		if got := checkChannelLicense(context.Background(), "test-device", "release", "family"); got != "error" {
			t.Fatalf("HTTP %d became %s", status, got)
		}
	}
}
