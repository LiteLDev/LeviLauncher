//go:build windows

package nativeinstall

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type licenseTransport func(*http.Request) (*http.Response, error)

func (f licenseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLicenseRejectsNoEntitlementAndTrial(t *testing.T) {
	for _, tc := range []struct{ name, body, code string }{
		{"no-entitlement", `{"satisfactionFailure":{"code":1,"description":"no entitlement"}}`, "ERR_LICENSE_NOT_ENTITLED"},
		{"trial", fmt.Sprintf(`{"license":{"keys":[{"value":%q}]}}`, base64.StdEncoding.EncodeToString([]byte(`<License><LicenseInfo Type="Trial"/><SPLicenseBlock/></License>`))), "ERR_LICENSE_FULL_REQUIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := http.DefaultTransport
			defer func() { http.DefaultTransport = original; active = nil; userTicket = ""; userReference = "" }()
			http.DefaultTransport = licenseTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "licensing.mp.microsoft.com" {
					t.Fatalf("unexpected request host %s", r.URL.Host)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})
			active = &installation{ctx: context.Background(), options: Options{RequireFullLicense: true, Market: "US"}}
			userTicket, userReference = "synthetic-ticket", "synthetic-user"
			err := caught(func() { license("synthetic-device", "synthetic-content") })
			if failure, ok := err.(*Error); !ok || failure.Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
			if len(contentKey) != 0 {
				t.Fatal("rejected license yielded a content key")
			}
		})
	}
}
