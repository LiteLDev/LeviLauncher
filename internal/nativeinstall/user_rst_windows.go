//go:build windows

// SPDX-License-Identifier: GPL-3.0-only
// User RST protocol adapted from Xodus 0670e25a; see THIRD_PARTY_NOTICES.
package nativeinstall

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"github.com/beevik/etree"
	"strings"
)

var deviceLegacy string
var deviceProof []byte

func reSignUser(doc *etree.Document, secret []byte) {
	sig := only(only(doc.Root(), "Header"), "Signature")
	si := only(sig, "SignedInfo")
	for _, ref := range si.ChildElements() {
		if ref.Tag != "Reference" {
			continue
		}
		id := strings.TrimPrefix(ref.SelectAttrValue("URI", ""), "#")
		var target *etree.Element
		for _, e := range allElements(doc.Root()) {
			if e.SelectAttrValue("Id", e.SelectAttrValue("wsu:Id", "")) == id {
				if target != nil {
					fail("duplicate signed target")
				}
				target = e
			}
		}
		if target == nil {
			fail("signed target missing")
		}
		digest := sha256.Sum256(canonical(target))
		only(ref, "DigestValue").SetText(base64.StdEncoding.EncodeToString(digest[:]))
	}
	nonce := decode64(only(only(doc.Root(), "DerivedKeyToken"), "Nonce").Text())
	key := derived(secret, nonce)
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	mac.Write(canonical(si))
	only(sig, "SignatureValue").SetText(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}
func makeUserRST(da map[string]string, finish, store bool) *etree.Document {
	scope := "scope=service::user.auth.xboxlive.com::MBI_SSL&api-version=2.0"
	policy := "TOKEN_BROKER"
	hosting := "000000004424da1f"
	if store {
		scope = "www.microsoft.com"
		policy = "mbi_ssl"
		hosting = "{d6d5a677-0872-4ab0-9442-bb792fce85c5}"
	}
	doc := makeRST("", scope, deviceLegacy, nil, deviceProof)
	header := only(doc.Root(), "Header")
	auth := only(header, "AuthInfo")
	only(auth, "HostingApp").SetText(hosting)
	auth.RemoveChild(only(auth, "LicenseSignatureKeyVersion"))
	if store {
		only(auth, "InlineUX").SetText("Silent")
	} else {
		add(auth, "ps:InlineFT", da["sSTSInlineFlowToken"])
	}
	security := only(header, "Security")
	var deviceEncrypted *etree.Element
	for _, e := range security.ChildElements() {
		if e.Tag == "EncryptedData" {
			deviceEncrypted = e
		}
	}
	if deviceEncrypted == nil {
		fail("device STS missing")
	}
	security.RemoveChild(deviceEncrypted)
	dt := add(security, "wsse:BinarySecurityToken", deviceLegacy)
	dt.CreateAttr("id", "DeviceDAToken")
	dt.CreateAttr("ValueType", "urn:liveid:device")
	user := security.CreateElement("wsse:UsernameToken")
	user.CreateAttr("wsu:Id", "user")
	add(user, "wsse:UsernameHint", da["sSigninName"])
	add(user, "wsse:LoginOption", "1")
	parsed := etree.NewDocument()
	must(parsed.ReadFromString(da["sDAToken"]))
	security.AddChild(parsed.Root().Copy())
	derivedToken := only(security, "DerivedKeyToken")
	reference := only(derivedToken, "RequestedTokenReference")
	reference.Space = "wsse"
	only(reference, "Reference").CreateAttr("URI", "#DeviceDAToken")
	rst := only(only(doc.Root(), "Body"), "RequestSecurityToken")
	only(rst, "PolicyReference").CreateAttr("URI", policy)
	if finish && !store {
		body := only(doc.Root(), "Body")
		body.RemoveChild(rst)
		multiple := body.CreateElement("ps:RequestMultipleSecurityTokens")
		multiple.CreateAttr("Id", "RSTS")
		multiple.AddChild(rst)
		second := multiple.CreateElement("wst:RequestSecurityToken")
		second.CreateAttr("Id", "RST1")
		add(second, "wst:RequestType", "http://schemas.xmlsoap.org/ws/2005/02/trust/Issue")
		add(second.CreateElement("wsp:AppliesTo").CreateElement("wsa:EndpointReference"), "wsa:Address", "http://Passport.NET/tb")
		for _, ref := range only(only(header, "Signature"), "SignedInfo").ChildElements() {
			if ref.Tag == "Reference" && ref.SelectAttrValue("URI", "") == "#RST0" {
				ref.CreateAttr("URI", "#RSTS")
			}
		}
	}
	reSignUser(doc, deviceProof)
	return doc
}
