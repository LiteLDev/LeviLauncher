//go:build windows

package nativeinstall

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/beevik/etree"
)

func encryptedRSTFixture(t *testing.T) (*etree.Document, []byte) {
	t.Helper()
	secret := bytes.Repeat([]byte{42}, 32)
	nonce := bytes.Repeat([]byte{17}, 32)
	key := derived(secret, nonce)
	doc := etree.NewDocument()
	root := doc.CreateElement("s:Envelope")
	for prefix, value := range ns {
		root.CreateAttr("xmlns:"+prefix, value)
	}
	root.CreateAttr("xmlns:xenc", "http://www.w3.org/2001/04/xmlenc#")
	header := root.CreateElement("s:Header")
	security := header.CreateElement("wsse:Security")
	dk := security.CreateElement("wssc:DerivedKeyToken")
	dk.CreateAttr("wsu:Id", "response-key")
	add(dk, "wssc:Nonce", base64.StdEncoding.EncodeToString(nonce))
	pp := header.CreateElement("ps:EncryptedPP")
	pp.CreateAttr("wsu:Id", "encrypted-pp")
	body := root.CreateElement("s:Body")
	body.CreateAttr("wsu:Id", "response-body")
	encrypt := func(parent *etree.Element, value string) {
		enc := parent.CreateElement("xenc:EncryptedData")
		ref := enc.CreateElement("KeyInfo").CreateElement("wsse:SecurityTokenReference").CreateElement("wsse:Reference")
		ref.CreateAttr("URI", "#response-key")
		plain := []byte(value)
		padding := aes.BlockSize - len(plain)%aes.BlockSize
		plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
		iv := bytes.Repeat([]byte{5}, aes.BlockSize)
		block, err := aes.NewCipher(key)
		if err != nil {
			t.Fatal(err)
		}
		data := make([]byte, len(plain))
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(data, plain)
		add(enc.CreateElement("xenc:CipherData"), "xenc:CipherValue", base64.StdEncoding.EncodeToString(append(iv, data...)))
	}
	encrypt(pp, `<ps:AuthInfo xmlns:ps="`+ns["ps"]+`"><ps:marker>header</ps:marker></ps:AuthInfo>`)
	encrypt(body, `<wst:RequestSecurityTokenResponse xmlns:wst="`+ns["wst"]+`"><wst:marker>body</wst:marker></wst:RequestSecurityTokenResponse>`)
	sig := security.CreateElement("Signature")
	sig.CreateAttr("xmlns", dsNS)
	si := sig.CreateElement("SignedInfo")
	for _, target := range []*etree.Element{pp, body} {
		ref := si.CreateElement("Reference")
		ref.CreateAttr("URI", "#"+target.SelectAttrValue("wsu:Id", ""))
		digest := sha256.Sum256(canonical(target))
		add(ref, "DigestValue", base64.StdEncoding.EncodeToString(digest[:]))
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(canonical(si))
	add(sig, "SignatureValue", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	sig.CreateElement("KeyInfo").CreateElement("wsse:SecurityTokenReference").CreateElement("wsse:Reference").CreateAttr("URI", "#response-key")
	return doc, secret
}

func TestRSTDecryptsBothEncryptedPPAndBody(t *testing.T) {
	doc, secret := encryptedRSTFixture(t)
	decryptRST(doc, secret)
	if len(walk(doc.Root(), "EncryptedPP")) != 0 || len(walk(doc.Root(), "EncryptedData")) != 0 {
		t.Fatal("an encrypted payload was not replaced")
	}
	if only(only(doc.Root(), "AuthInfo"), "marker").Text() != "header" {
		t.Fatal("header payload missing")
	}
	if only(only(doc.Root(), "RequestSecurityTokenResponse"), "marker").Text() != "body" {
		t.Fatal("body payload missing")
	}
}

func TestRSTRejectsCiphertextTamperingBeforeDecrypting(t *testing.T) {
	doc, secret := encryptedRSTFixture(t)
	value := walk(doc.Root(), "CipherValue")[0]
	data := decode64(value.Text())
	data[20] ^= 1
	value.SetText(base64.StdEncoding.EncodeToString(data))
	if caught(func() { decryptRST(doc, secret) }) == nil {
		t.Fatal("tampered signed payload was accepted")
	}
}
