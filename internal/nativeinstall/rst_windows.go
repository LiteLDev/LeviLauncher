// SPDX-License-Identifier: GPL-3.0-only
// RST protocol adapted from Xodus 0670e25aeb0e0e9f800f8f2f4968ae3b681842a7.
package nativeinstall

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"golang.org/x/sys/windows"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

const dsNS = "http://www.w3.org/2000/09/xmldsig#"
const exNS = "http://www.w3.org/2001/10/xml-exc-c14n#"

var ns = map[string]string{"s": "http://www.w3.org/2003/05/soap-envelope", "ps": "http://schemas.microsoft.com/Passport/SoapServices/PPCRL", "wsse": "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd", "wsu": "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd", "wsa": "http://www.w3.org/2005/08/addressing", "wssc": "http://schemas.xmlsoap.org/ws/2005/02/sc", "wst": "http://schemas.xmlsoap.org/ws/2005/02/trust", "wsp": "http://schemas.xmlsoap.org/ws/2004/09/policy", "saml": "urn:oasis:names:tc:SAML:1.0:assertion"}

type ownDevice struct{ Member, Password, PUID, License string }

var ownKey []byte
var ownDeviceID []byte

func loadProtected(path string, out any) error {
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	if info.Size() == 0 || info.Size() > 1<<20 {
		return fmt.Errorf("invalid device cache size")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if len(b) == 0 {
		return fmt.Errorf("empty protected cache")
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var plain windows.DataBlob
	e = windows.CryptUnprotectData(&in, nil, nil, 0, nil, 1, &plain)
	if e != nil {
		return e
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(plain.Data))))
	p := unsafe.Slice(plain.Data, plain.Size)
	defer clear(p)
	return json.Unmarshal(p, out)
}
func saveProtected(path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	defer clear(b)
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	e = windows.CryptProtectData(&in, nil, nil, 0, nil, 1, &out)
	if e != nil {
		return e
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	temp, e := os.CreateTemp(filepath.Dir(path), ".device-*.dpapi")
	if e != nil {
		return e
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, e = temp.Write(unsafe.Slice(out.Data, out.Size)); e != nil {
		temp.Close()
		return e
	}
	if e = temp.Sync(); e != nil {
		temp.Close()
		return e
	}
	if e = temp.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
func parseSP(b []byte) map[uint32][]byte {
	if len(b) < 8 || len(b) > 1<<20 {
		fail("invalid SPLicense size")
	}
	r := map[uint32][]byte{}
	for p := 8; p < len(b); {
		if len(b)-p < 8 {
			fail("truncated TLV header")
		}
		id, n := binary.LittleEndian.Uint32(b[p:]), int(binary.LittleEndian.Uint32(b[p+4:]))
		p += 8
		if n > len(b)-p {
			fail("truncated TLV data")
		}
		if _, ok := r[id]; ok {
			fail("duplicate TLV")
		}
		r[id] = b[p : p+n]
		p += n
	}
	return r
}
func decode64(s string) []byte {
	b, e := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	must(e)
	return b
}

// CLEP encoding offsets and derivation are from the MIT-licensed SPLicense work credited by Xodus to LukeFZ.
func scheduleKey(s []byte) []byte {
	if len(s) < 232 {
		fail("short CLEP schedule")
	}
	word := func(i int) uint32 { return binary.LittleEndian.Uint32(s[i*4:]) }
	out := make([]byte, 16)
	for i, w := range []uint32{word(46) ^ word(56) ^ 0xe20df371 ^ 0xccb22fe6, word(36) ^ word(47) ^ 0xdf080e39, word(40) ^ word(51) ^ 0x6d09b2f5 ^ 0x2ae17ab9, word(30) ^ word(41) ^ 0x37288cec} {
		binary.LittleEndian.PutUint32(out[i*4:], w)
	}
	return out
}
func cbc(key, iv, data []byte) []byte {
	if len(iv) != 16 || len(data) == 0 || len(data)%16 != 0 {
		fail("invalid CBC input")
	}
	a, e := aes.NewCipher(key)
	must(e)
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(a, iv).CryptBlocks(out, data)
	return out
}
func clep(b []byte, n int) []byte {
	if len(b) != 4096 || binary.LittleEndian.Uint32(b) != 4 {
		fail("unsupported CLEP state")
	}
	k := scheduleKey(b[4+n:])
	defer clear(k)
	return cbc(k, make([]byte, 16), b[4:4+n])
}
func deviceKey(b []byte) []byte {
	if len(b) != 4096 || binary.LittleEndian.Uint16(b) != 4096 || binary.LittleEndian.Uint32(b[2:]) != 4 {
		fail("unsupported device wrapping key")
	}
	key := scheduleKey(b[6:])
	check := cbc(key, make([]byte, 16), b[518:534])
	defer clear(check)
	if !hmac.Equal(key, check) {
		clear(key)
		fail("device wrapping key check failed")
	}
	return key
}
func rsaKey(b []byte) *rsa.PrivateKey {
	if len(b) < 24 || binary.LittleEndian.Uint32(b) != 0x32415352 {
		fail("invalid BCrypt RSA private blob")
	}
	pos := 24
	read := func(n int) *big.Int {
		if n < 1 || n > len(b)-pos {
			fail("invalid RSA field")
		}
		r := new(big.Int).SetBytes(b[pos : pos+n])
		pos += n
		return r
	}
	e := read(int(binary.LittleEndian.Uint32(b[8:])))
	n := read(int(binary.LittleEndian.Uint32(b[12:])))
	p := read(int(binary.LittleEndian.Uint32(b[16:])))
	q := read(int(binary.LittleEndian.Uint32(b[20:])))
	one := big.NewInt(1)
	phi := new(big.Int).Mul(new(big.Int).Sub(p, one), new(big.Int).Sub(q, one))
	d := new(big.Int).ModInverse(e, phi)
	if d == nil {
		fail("invalid RSA inverse")
	}
	key := &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: n, E: int(e.Int64())}, D: d, Primes: []*big.Int{p, q}}
	must(key.Validate())
	key.Precompute()
	return key
}
func add(p *etree.Element, name, value string) *etree.Element {
	e := p.CreateElement(name)
	e.SetText(value)
	return e
}
func canonical(e *etree.Element) []byte {
	copy := e.Copy()
	seen := map[string]bool{}
	for p := e; p != nil; p = p.Parent() {
		for _, a := range p.Attr {
			if a.Space == "xmlns" || a.Key == "xmlns" {
				name := a.Key
				if a.Space != "" {
					name = a.Space + ":" + a.Key
				}
				if !seen[name] {
					copy.CreateAttr(name, a.Value)
					seen[name] = true
				}
			}
		}
	}
	b, err := dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("").Canonicalize(copy)
	must(err)
	return b
}
func walk(e *etree.Element, name string) []*etree.Element {
	r := []*etree.Element{}
	if e.Tag == name {
		r = append(r, e)
	}
	for _, c := range e.ChildElements() {
		r = append(r, walk(c, name)...)
	}
	return r
}
func only(e *etree.Element, name string) *etree.Element {
	r := walk(e, name)
	if len(r) != 1 {
		fail(fmt.Sprintf("expected one %s, got %d", name, len(r)))
	}
	return r[0]
}
func derived(key, nonce []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte{0, 0, 0, 1})
	m.Write([]byte("WS-SecureConversationWS-SecureConversation"))
	m.Write([]byte{0})
	m.Write(nonce)
	m.Write([]byte{0, 0, 1, 0})
	return m.Sum(nil)
}
func makeRST(member, scope, ticket string, rkey *rsa.PrivateKey, secret []byte) *etree.Document {
	doc := etree.NewDocument()
	root := doc.CreateElement("s:Envelope")
	for k, v := range ns {
		root.CreateAttr("xmlns:"+k, v)
	}
	header := root.CreateElement("s:Header")
	add(header, "wsa:Action", "http://schemas.xmlsoap.org/ws/2005/02/trust/RST/Issue").CreateAttr("s:mustUnderstand", "1")
	add(header, "wsa:To", "https://login.live.com:443/RST2.srf").CreateAttr("s:mustUnderstand", "1")
	add(header, "wsa:MessageID", fmt.Sprint(time.Now().Unix()))
	info := header.CreateElement("ps:AuthInfo")
	info.CreateAttr("Id", "PPAuthInfo")
	hosting := "{DF60E2DF-88AD-4526-AE21-83D130EF0F68}"
	flags := ""
	if ticket != "" {
		hosting = "{d6d5a677-0872-4ab0-9442-bb792fce85c5}"
		flags = "SsoRestr"
	}
	for _, kv := range [][2]string{{"SSOFlags", flags}, {"HostingApp", hosting}, {"BinaryVersion", "55"}, {"UIVersion", "1"}, {"InlineUX", "TokenBroker"}, {"IsAdmin", "1"}, {"Cookies", ""}, {"RequestParams", "AQAAAAIAAABsYwQAAAAxMDMz"}, {"WindowsClientString", "b4d/QB7Zy5pjUAY9ByQ1echTyTITx6ZCErOEztuIVtw="}, {"LicenseSignatureKeyVersion", "2"}, {"ClientCapabilities", "1"}} {
		add(info, "ps:"+kv[0], kv[1])
	}
	security := header.CreateElement("wsse:Security")
	nonce := make([]byte, 32)
	mustReadRandom(nonce)
	if ticket == "" {
		user := security.CreateElement("wsse:UsernameToken")
		user.CreateAttr("wsu:Id", "devicesoftware")
		add(user, "wsse:Username", member)
	} else {
		td := etree.NewDocument()
		must(td.ReadFromString(ticket))
		security.AddChild(td.Root().Copy())
		dk := security.CreateElement("wssc:DerivedKeyToken")
		dk.CreateAttr("wsu:Id", "SignKey")
		dk.CreateAttr("Algorithm", "urn:liveid:SP800108_CTR_HMAC_SHA256_DOUBLEDERIVED")
		ref := dk.CreateElement("RequestedTokenReference")
		ki := add(ref, "wsse:KeyIdentifier", "")
		ki.CreateAttr("ValueType", "http://docs.oasis-open.org/wss/2004/XX/oasis-2004XX-wss-saml-token-profile-1.0#SAMLAssertionID")
		ref.CreateElement("wsse:Reference").CreateAttr("URI", "")
		add(dk, "wssc:Nonce", base64.StdEncoding.EncodeToString(nonce))
	}
	ts := security.CreateElement("wsu:Timestamp")
	ts.CreateAttr("wsu:Id", "Timestamp")
	now := time.Now().UTC()
	add(ts, "wsu:Created", now.Format(time.RFC3339))
	add(ts, "wsu:Expires", now.Add(5*time.Minute).Format(time.RFC3339))
	body := root.CreateElement("s:Body")
	rst := body.CreateElement("wst:RequestSecurityToken")
	rst.CreateAttr("Id", "RST0")
	add(rst, "wst:RequestType", "http://schemas.xmlsoap.org/ws/2005/02/trust/Issue")
	at := rst.CreateElement("wsp:AppliesTo")
	add(at.CreateElement("wsa:EndpointReference"), "wsa:Address", scope)
	if scope != "http://Passport.NET/tb" {
		rst.CreateElement("wsp:PolicyReference").CreateAttr("URI", "MBI_SSL")
	}
	sig := security.CreateElement("Signature")
	sig.CreateAttr("xmlns", dsNS)
	si := sig.CreateElement("SignedInfo")
	si.CreateElement("CanonicalizationMethod").CreateAttr("Algorithm", exNS)
	method := "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	if ticket != "" {
		method = "http://www.w3.org/2001/04/xmldsig-more#hmac-sha256"
	}
	si.CreateElement("SignatureMethod").CreateAttr("Algorithm", method)
	for _, target := range []*etree.Element{rst, ts, info} {
		id := target.SelectAttrValue("Id", target.SelectAttrValue("wsu:Id", ""))
		ref := si.CreateElement("Reference")
		ref.CreateAttr("URI", "#"+id)
		ref.CreateElement("Transforms").CreateElement("Transform").CreateAttr("Algorithm", exNS)
		ref.CreateElement("DigestMethod").CreateAttr("Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
		sum := sha256.Sum256(canonical(target))
		add(ref, "DigestValue", base64.StdEncoding.EncodeToString(sum[:]))
	}
	signed := canonical(si)
	var signature []byte
	if rkey != nil {
		sum := sha256.Sum256(signed)
		var e error
		signature, e = rsa.SignPKCS1v15(rand.Reader, rkey, crypto.SHA256, sum[:])
		must(e)
	} else {
		k := derived(secret, nonce)
		defer clear(k)
		mac := hmac.New(sha256.New, k)
		mac.Write(signed)
		signature = mac.Sum(nil)
	}
	add(sig, "SignatureValue", base64.StdEncoding.EncodeToString(signature))
	if ticket != "" {
		sig.CreateElement("KeyInfo").CreateElement("wsse:SecurityTokenReference").CreateElement("wsse:Reference").CreateAttr("URI", "#SignKey")
	}
	return doc
}
func mustReadRandom(b []byte) { _, e := rand.Read(b); must(e) }
func postRST(doc *etree.Document, secret []byte) *etree.Document {
	b := canonical(doc.Root())
	defer clear(b)
	client := http.Client{Timeout: 35 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(active.ctx, "POST", "https://login.live.com/RST2.srf", bytes.NewReader(b))
	must(e)
	req.Header.Set("Content-Type", "application/soap+xml")
	req.Header.Set("User-Agent", "MSAWindows/55 (OS 10.0.26100.0.0 ge_release; IDK 10.0.26100.5074 ge_release; Cfg 16.000.29325.00; Test 0)")
	res, e := client.Do(req)
	must(e)
	defer res.Body.Close()
	raw := boundedBody(res.Body, 2<<20)
	defer clear(raw)
	out := etree.NewDocument()
	must(out.ReadFromBytes(raw))
	emit(map[string]any{"rst_http_status": res.StatusCode, "rst_response_bytes": len(raw), "rst_fault_count": len(walk(out.Root(), "Fault")), "rst_requested_token_count": len(walk(out.Root(), "RequestedSecurityToken"))})
	if len(secret) > 0 {
		if len(walk(only(out.Root(), "Header"), "Signature")) == 1 {
			decryptRST(out, secret)
		} else if len(walk(out.Root(), "Fault")) == 0 {
			fail("unsigned RST success response")
		}
	}
	return out
}
func decryptRST(doc *etree.Document, secret []byte) {
	nonces := map[string][]byte{}
	for _, dk := range walk(doc.Root(), "DerivedKeyToken") {
		id := dk.SelectAttrValue("wsu:Id", dk.SelectAttrValue("Id", ""))
		nonces[id] = decode64(only(dk, "Nonce").Text())
	}
	signatures := walk(only(doc.Root(), "Header"), "Signature")
	if len(signatures) != 1 {
		fail("missing/ambiguous RST signature")
	}
	sig := signatures[0]
	ref := only(only(sig, "KeyInfo"), "Reference")
	nonce, ok := nonces[strings.TrimPrefix(ref.SelectAttrValue("URI", ""), "#")]
	if !ok {
		fail("RST signature nonce missing")
	}
	key := derived(secret, nonce)
	defer clear(key)
	si := only(sig, "SignedInfo")
	mac := hmac.New(sha256.New, key)
	mac.Write(canonical(si))
	if !hmac.Equal(mac.Sum(nil), decode64(only(sig, "SignatureValue").Text())) {
		fail("RST signature failed")
	}
	for _, r := range si.ChildElements() {
		if r.Tag != "Reference" {
			continue
		}
		id := strings.TrimPrefix(r.SelectAttrValue("URI", ""), "#")
		targets := []*etree.Element{}
		for _, el := range allElements(doc.Root()) {
			if el.SelectAttrValue("wsu:Id", el.SelectAttrValue("Id", "")) == id {
				targets = append(targets, el)
			}
		}
		if len(targets) != 1 {
			fail("RST signed target ambiguous")
		}
		sum := sha256.Sum256(canonical(targets[0]))
		if !hmac.Equal(sum[:], decode64(only(r, "DigestValue").Text())) {
			fail("RST signed digest failed")
		}
	}
	for _, enc := range walk(doc.Root(), "EncryptedData") {
		parent := enc.Parent()
		if parent == nil || (parent.Tag != "Body" && parent.Tag != "EncryptedPP") {
			continue
		}
		ref := only(only(enc, "KeyInfo"), "Reference")
		nonce, ok := nonces[strings.TrimPrefix(ref.SelectAttrValue("URI", ""), "#")]
		if !ok {
			fail("RST encryption nonce missing")
		}
		key := derived(secret, nonce)
		ciphertext := decode64(only(enc, "CipherValue").Text())
		if len(ciphertext) < 32 {
			fail("short encrypted RST")
		}
		plain := cbc(key, ciphertext[:16], ciphertext[16:])
		clear(key)
		padding := int(plain[len(plain)-1])
		if padding < 1 || padding > 16 || padding > len(plain) {
			fail("invalid RST padding")
		}
		for _, v := range plain[len(plain)-padding:] {
			if int(v) != padding {
				fail("invalid RST padding")
			}
		}
		tmp := etree.NewDocument()
		must(tmp.ReadFromBytes(plain[:len(plain)-padding]))
		clear(plain)
		if parent.Tag == "Body" {
			parent.RemoveChild(enc)
			parent.AddChild(tmp.Root().Copy())
		} else {
			header := parent.Parent()
			header.RemoveChild(parent)
			header.AddChild(tmp.Root().Copy())
		}
	}
	emit(map[string]any{"rst_response_signature_verified": true, "rst_payload_decrypted": true})
}

func allElements(e *etree.Element) []*etree.Element {
	out := []*etree.Element{e}
	for _, c := range e.ChildElements() {
		out = append(out, allElements(c)...)
	}
	return out
}
func ownDeviceTicket() string {
	var state ownDevice
	must(loadProtected(cachePath(), &state))
	blocks := parseSP(decode64(state.License))
	validateDevice(state)
	ownKey = deviceKey(blocks[1])
	encodedID := blocks[2]
	if len(encodedID) != 10 || binary.LittleEndian.Uint16(encodedID) != 8 {
		fail("unsupported device ID encoding")
	}
	ownDeviceID = append([]byte(nil), encodedID[2:]...)
	emit(map[string]any{"device_license_blocks": func() map[string]int {
		r := map[string]int{}
		for id, b := range blocks {
			r[fmt.Sprintf("0x%x", id)] = len(b)
		}
		return r
	}()})
	priv := clep(blocks[0x12d], 544)
	defer clear(priv)
	rsa := rsaKey(priv)
	first := postRST(makeRST(state.Member, "http://Passport.NET/tb", "", rsa, nil), nil)
	tokens := walk(first.Root(), "RequestedSecurityToken")
	if len(tokens) != 1 || len(tokens[0].ChildElements()) != 1 {
		fail("missing device STS")
	}
	td := etree.NewDocument()
	td.SetRoot(tokens[0].ChildElements()[0].Copy())
	ticket, e := td.WriteToString()
	must(e)
	proof := only(first.Root(), "BinarySecret").Text()
	secretData := clep(decode64(proof), 48)
	defer clear(secretData)
	secret := secretData[12:44]
	deviceLegacy = ticket
	clear(deviceProof)
	deviceProof = append([]byte(nil), secret...)
	second := postRST(makeRST("", "www.microsoft.com", ticket, nil, secret), secret)
	token := only(only(second.Root(), "RequestedSecurityToken"), "BinarySecurityToken").Text()
	emit(map[string]any{"own_device_ms_ticket_chars": len(token), "own_device_wrapping_key_validated": true})
	return token
}
