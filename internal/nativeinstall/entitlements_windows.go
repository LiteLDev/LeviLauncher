//go:build windows

// SPDX-License-Identifier: GPL-3.0-only
package nativeinstall

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// License checks return only display states, never credentials or license keys.
type GameLicenses struct {
	XUID    string `json:"xuid"`
	Release string `json:"release"`
	Preview string `json:"preview"`
}

type catalogProduct struct {
	Product struct {
		ProductID                string
		DisplaySkuAvailabilities []struct {
			Sku struct {
				Properties struct {
					IsTrial    bool
					IsPreOrder bool
					Packages   []struct {
						ContentID         string
						PackageFormat     string
						PackageFamilyName string
						Architectures     []string
					}
				}
			}
		}
	}
}

// Use the current public catalog rather than hardcoding version-specific content IDs.
func catalogContentID(ctx context.Context, productID, family string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://displaycatalog.mp.microsoft.com/v7.0/products/"+productID+"?market=US&languages=en-us", nil)
	must(err)
	res, err := httpClient().Do(req)
	must(err)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		failCode("ERR_LICENSE_CATALOG", "catalog request failed")
	}
	var catalog catalogProduct
	must(json.Unmarshal(boundedBody(res.Body, 8<<20), &catalog))
	return selectCatalogContentID(catalog, productID, family)
}

func selectCatalogContentID(catalog catalogProduct, productID, family string) string {
	if catalog.Product.ProductID != productID {
		failCode("ERR_LICENSE_CATALOG", "catalog product mismatch")
	}
	for _, item := range catalog.Product.DisplaySkuAvailabilities {
		p := item.Sku.Properties
		if p.IsTrial || p.IsPreOrder {
			continue
		}
		for _, pkg := range p.Packages {
			if pkg.PackageFormat != "MSIXVC" || pkg.PackageFamilyName != family {
				continue
			}
			for _, arch := range pkg.Architectures {
				if arch == "x64" && validContentID(pkg.ContentID) {
					return pkg.ContentID
				}
			}
		}
	}
	failCode("ERR_LICENSE_CATALOG", "no supported retail package")
	return ""
}

func validContentID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func contentLicenseState(parsed contentLicenseResponse) string {
	// A satisfaction failure means this request was not granted. Do not claim
	// permanent ownership or infer it from catalog availability / Xbox sign-in.
	if parsed.SatisfactionFailure != nil {
		return "not_entitled"
	}
	if parsed.License == nil || len(parsed.License.Keys) == 0 || len(parsed.License.Keys) > 16 {
		return "error"
	}
	state := "error"
	for _, record := range parsed.License.Keys {
		data, err := base64.StdEncoding.DecodeString(record.Value)
		if err != nil {
			return "error"
		}
		doc := etree.NewDocument()
		err = doc.ReadFromBytes(data)
		clear(data)
		if err != nil || doc.Root() == nil {
			return "error"
		}
		infos := doc.Root().SelectElements("LicenseInfo")
		if len(infos) != 1 {
			return "error"
		}
		switch infos[0].SelectAttrValue("Type", "") {
		case "Full":
			state = "authorized"
		case "Trial":
			if state != "authorized" {
				state = "trial"
			}
		default:
			return "error"
		}
	}
	return state
}

func checkChannelLicense(ctx context.Context, device, productID, family string) (state string) {
	state = "error"
	// Keep one channel's network/catalog failure from suppressing the other.
	var err error
	defer func() {
		if err != nil {
			state = "error"
		}
	}()
	defer recoverFailure(&err)
	id := catalogContentID(ctx, productID, family)
	return contentLicenseState(requestContentLicense(device, id))
}

func CheckGameLicenses(ctx context.Context, dir, expectedXUID string) GameLicenses {
	result := GameLicenses{XUID: expectedXUID, Release: "error", Preview: "error"}
	if expectedXUID == "" {
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	_ = accountOperation(ctx, dir, func() {
		active.options.Market = "US"
		acquireUserTicket(ctx, func(ctx context.Context) (string, string, error) {
			return xbox.GetStoreTicketForUser(ctx, expectedXUID)
		})
		ensureDevice()
		device := ownDeviceTicket()
		result.Release = checkChannelLicense(ctx, device, "9NBLGGH2JHXJ", "Microsoft.MinecraftUWP_8wekyb3d8bbwe")
		result.Preview = checkChannelLicense(ctx, device, "9P5X4QVLC2XR", "Microsoft.MinecraftWindowsBeta_8wekyb3d8bbwe")
	})
	return result
}
