package uwp

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// An Appx package signature (AppxSignature.p7x) is "PKCX" followed by an
// Authenticode PKCS#7 SignedData. Its SpcIndirectDataContent digest is
// "APPX" followed by tagged SHA-256 digests of the package parts:
//
//	AXPC  local file records      AXCD  central directory
//	AXCT  [Content_Types].xml     AXBM  AppxBlockMap.xml
//	AXCI  AppxMetadata/CodeIntegrity.cat (optional)
//
// A delta upgrade never has the whole archive, so it cannot use the file
// digest from Windows Update. It verifies this signature instead: once the
// signer chains to Microsoft, AXBM authenticates the block map, and the block
// map authenticates every byte of every payload file.

var (
	oidSignedData       = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSpcIndirectData  = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 4}
	oidAttrContentType  = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidAttrMessageDigst = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSHA256           = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSAEncryption    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidSHA256WithRSA    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
)

type p7ContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,tag:0"`
}

type p7SignedData struct {
	Version          int
	DigestAlgorithms asn1.RawValue
	ContentInfo      p7EncapContent
	Certificates     asn1.RawValue  `asn1:"optional,tag:0"`
	CRLs             asn1.RawValue  `asn1:"optional,tag:1"`
	SignerInfos      []p7SignerInfo `asn1:"set"`
}

type p7EncapContent struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,tag:0"`
}

type p7IssuerAndSerial struct {
	Issuer asn1.RawValue
	Serial *big.Int
}

type p7SignerInfo struct {
	Version                   int
	IssuerAndSerial           p7IssuerAndSerial
	DigestAlgorithm           pkix.AlgorithmIdentifier
	AuthenticatedAttributes   asn1.RawValue `asn1:"optional,tag:0"`
	DigestEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedDigest           []byte
	UnauthenticatedAttributes asn1.RawValue `asn1:"optional,tag:1"`
}

type p7Attribute struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue `asn1:"set"`
}

type spcIndirectData struct {
	Data struct {
		Type  asn1.ObjectIdentifier
		Value asn1.RawValue `asn1:"optional"`
	}
	MessageDigest struct {
		Algorithm pkix.AlgorithmIdentifier
		Digest    []byte
	}
}

// appxDigests are the verified package part digests, keyed by tag ("AXBM").
type appxDigests map[string][]byte

// verifyAppxSignature checks a package signature and returns the digests it
// signs. The signer must chain to a root trusted by Windows and be Microsoft.
// Certificate lifetimes are checked at the signing certificate's start time,
// as old game builds are signed by certificates that have long expired.
func verifyAppxSignature(p7x []byte) (appxDigests, *x509.Certificate, error) {
	if len(p7x) < 8 || string(p7x[:4]) != "PKCX" {
		return nil, nil, errors.New("not an Appx signature")
	}
	var ci p7ContentInfo
	if rest, err := asn1.Unmarshal(p7x[4:], &ci); err != nil || len(rest) != 0 {
		return nil, nil, fmt.Errorf("signature content: %v", err)
	}
	if !ci.ContentType.Equal(oidSignedData) {
		return nil, nil, errors.New("signature is not SignedData")
	}
	var sd p7SignedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		return nil, nil, fmt.Errorf("signed data: %v", err)
	}
	if !sd.ContentInfo.ContentType.Equal(oidSpcIndirectData) {
		return nil, nil, errors.New("signature is not Authenticode")
	}
	if len(sd.SignerInfos) != 1 {
		return nil, nil, errors.New("expected exactly one signer")
	}
	si := sd.SignerInfos[0]

	// Signed content: the SpcIndirectDataContent. Authenticode hashes its
	// value without the outer SEQUENCE tag and length.
	var content asn1.RawValue
	if _, err := asn1.Unmarshal(sd.ContentInfo.Content.Bytes, &content); err != nil {
		return nil, nil, fmt.Errorf("indirect data: %v", err)
	}
	var indirect spcIndirectData
	if _, err := asn1.Unmarshal(content.FullBytes, &indirect); err != nil {
		return nil, nil, fmt.Errorf("indirect data: %v", err)
	}
	if !si.DigestAlgorithm.Algorithm.Equal(oidSHA256) {
		return nil, nil, errors.New("unsupported signature digest")
	}
	contentHash := sha256.Sum256(content.Bytes)

	// The authenticated attributes carry the content digest and are what the
	// signature covers, DER-encoded as a SET.
	if len(si.AuthenticatedAttributes.FullBytes) == 0 {
		return nil, nil, errors.New("signature has no authenticated attributes")
	}
	var attrs []p7Attribute
	if _, err := asn1.UnmarshalWithParams(si.AuthenticatedAttributes.FullBytes, &attrs, "set,tag:0"); err != nil {
		return nil, nil, fmt.Errorf("authenticated attributes: %v", err)
	}
	var gotDigest []byte
	contentTypeOK := false
	for _, a := range attrs {
		switch {
		case a.Type.Equal(oidAttrMessageDigst):
			if _, err := asn1.Unmarshal(a.Values.Bytes, &gotDigest); err != nil {
				return nil, nil, fmt.Errorf("message digest attribute: %v", err)
			}
		case a.Type.Equal(oidAttrContentType):
			var oid asn1.ObjectIdentifier
			if _, err := asn1.Unmarshal(a.Values.Bytes, &oid); err == nil && oid.Equal(oidSpcIndirectData) {
				contentTypeOK = true
			}
		}
	}
	if !contentTypeOK || !bytes.Equal(gotDigest, contentHash[:]) {
		return nil, nil, errors.New("signed content digest mismatch")
	}
	signedAttrs := append([]byte{0x31}, si.AuthenticatedAttributes.FullBytes[1:]...)

	// Certificates and signer.
	var certs []*x509.Certificate
	for rest := sd.Certificates.Bytes; len(rest) > 0; {
		var raw asn1.RawValue
		var err error
		if rest, err = asn1.Unmarshal(rest, &raw); err != nil {
			return nil, nil, fmt.Errorf("certificates: %v", err)
		}
		if cert, err := x509.ParseCertificate(raw.FullBytes); err == nil {
			certs = append(certs, cert)
		}
	}
	var signer *x509.Certificate
	for _, c := range certs {
		if bytes.Equal(c.RawIssuer, si.IssuerAndSerial.Issuer.FullBytes) && c.SerialNumber.Cmp(si.IssuerAndSerial.Serial) == 0 {
			signer = c
			break
		}
	}
	if signer == nil {
		return nil, nil, errors.New("signer certificate not found")
	}
	alg := si.DigestEncryptionAlgorithm.Algorithm
	if !alg.Equal(oidRSAEncryption) && !alg.Equal(oidSHA256WithRSA) {
		return nil, nil, errors.New("unsupported signature algorithm")
	}
	if err := signer.CheckSignature(x509.SHA256WithRSA, signedAttrs, si.EncryptedDigest); err != nil {
		return nil, nil, fmt.Errorf("signature: %v", err)
	}

	intermediates := x509.NewCertPool()
	for _, c := range certs {
		if c != signer {
			intermediates.AddCert(c)
		}
	}
	chains, err := signer.Verify(x509.VerifyOptions{
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		CurrentTime:   signer.NotBefore.Add(time.Minute),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("certificate chain: %v", err)
	}
	if !isMicrosoftChain(signer, chains) {
		return nil, nil, errors.New("package is not signed by Microsoft")
	}

	digest := indirect.MessageDigest.Digest
	if len(digest) < 4 || string(digest[:4]) != "APPX" {
		return nil, nil, errors.New("signature does not sign an Appx package")
	}
	out := appxDigests{}
	for rest := digest[4:]; len(rest) > 0; rest = rest[36:] {
		if len(rest) < 36 {
			return nil, nil, errors.New("truncated Appx digest")
		}
		out[string(rest[:4])] = append([]byte(nil), rest[4:36]...)
	}
	if len(out["AXBM"]) != sha256.Size || len(out["AXCT"]) != sha256.Size {
		return nil, nil, errors.New("Appx digest lacks block map or content types")
	}
	return out, signer, nil
}

func isMicrosoftChain(signer *x509.Certificate, chains [][]*x509.Certificate) bool {
	if !containsFold(signer.Subject.Organization, "Microsoft Corporation") {
		return false
	}
	for _, chain := range chains {
		root := chain[len(chain)-1]
		if containsFold(root.Subject.Organization, "Microsoft Corporation") &&
			strings.Contains(strings.ToLower(root.Subject.CommonName), "microsoft root") {
			return true
		}
	}
	return false
}

func containsFold(values []string, want string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return true
		}
	}
	return false
}
