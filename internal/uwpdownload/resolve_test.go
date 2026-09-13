package uwpdownload

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testLocationDigest = "qUqP5cyxm6YcTAhz05Hph5gvu9M="

func locations(url string) string {
	return `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><GetExtendedUpdateInfo2Response><GetExtendedUpdateInfo2Result><FileLocations><FileLocation><FileDigest>qUqP5cyxm6YcTAhz05Hph5gvu9M=</FileDigest><Url>` + url + `</Url></FileLocation></FileLocations></GetExtendedUpdateInfo2Result></GetExtendedUpdateInfo2Response></s:Body></s:Envelope>`
}

func TestResolveWindowsUpdateRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		for _, required := range []string{testID, "<RevisionNumber>1</RevisionNumber>", "FileUrl", "GetExtendedUpdateInfo2"} {
			if !strings.Contains(string(body), required) {
				t.Errorf("missing request fragment %q", required)
			}
		}
		if r.Method != http.MethodPost || !strings.Contains(r.Header.Get("Content-Type"), "application/soap+xml") {
			t.Error("incorrect SOAP request")
		}
		io.WriteString(w, locations("http://tlu.dl.delivery.mp.microsoft.com/filestreaming/files/test?P1=1&amp;P2=two"))
	}))
	defer srv.Close()
	got, err := resolveURL(context.Background(), srv.Client(), srv.URL, testID)
	if err != nil || got.URL != "http://tlu.dl.delivery.mp.microsoft.com/filestreaming/files/test?P1=1&P2=two" || len(got.Digest) != 20 {
		t.Fatalf("resolve: %+v %v", got, err)
	}
}

func TestRejectUntrustedDownloadLocation(t *testing.T) {
	for _, u := range []string{"https://example.com/game.appx", "http://tlu.dl.delivery.mp.microsoft.com.evil.test/a", "file:///c:/test", "https://user@tlu.dl.delivery.mp.microsoft.com/a"} {
		if _, err := parseLocations(strings.NewReader(locations(u))); err == nil {
			t.Errorf("accepted %q", u)
		}
	}
	if _, err := ResolveURL(context.Background(), `bad<&id`); err == nil {
		t.Fatal("accepted invalid update ID")
	}
}

func TestLocationsRequireAuthenticatedPayloadDigest(t *testing.T) {
	payload := []byte("Minecraft archive")
	sha1Digest := sha1.Sum(payload)
	sha256Digest := sha256.Sum256(payload)
	for _, tc := range []struct {
		name   string
		digest []byte
	}{
		{"SHA1", sha1Digest[:]},
		{"SHA256", sha256Digest[:]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, scheme := range []string{"http", "https"} {
				url := scheme + "://tlu.dl.delivery.mp.microsoft.com/filestreaming/files/game?P1=1&P2=signed%2Bquery"
				xml := strings.Replace(locations(strings.ReplaceAll(url, "&", "&amp;")), testLocationDigest, base64.StdEncoding.EncodeToString(tc.digest), 1)
				got, err := parseLocations(strings.NewReader(xml))
				if err != nil || got.URL != url || !bytes.Equal(got.Digest, tc.digest) {
					t.Fatalf("payload location/digest changed: got %+v, error %v", got, err)
				}
			}
		})
	}
	for _, digest := range []string{"", "not!base64", base64.StdEncoding.EncodeToString(make([]byte, 16)), base64.StdEncoding.EncodeToString(make([]byte, 31))} {
		xml := strings.Replace(locations("http://tlu.dl.delivery.mp.microsoft.com/game"), testLocationDigest, digest, 1)
		if _, err := parseLocations(strings.NewReader(xml)); err == nil {
			t.Fatalf("accepted missing/invalid digest %q", digest)
		}
	}
}

func TestLocationsIgnoreAuxiliaryFilesAndBindDigestToPayload(t *testing.T) {
	payloadDigest := sha256.Sum256([]byte("actual package"))
	auxiliary := `<FileLocation><FileDigest>` + testLocationDigest + `</FileDigest><Url>http://dl.delivery.mp.microsoft.com/auxiliary</Url></FileLocation>`
	invalidPayload := `<FileLocation><Url>http://tlu.dl.delivery.mp.microsoft.com/without-digest</Url></FileLocation>`
	response := strings.Replace(locations("http://tlu.dl.delivery.mp.microsoft.com/package"), testLocationDigest, base64.StdEncoding.EncodeToString(payloadDigest[:]), 1)
	response = strings.Replace(response, "<FileLocations>", "<FileLocations>"+auxiliary+invalidPayload, 1)
	got, err := parseLocations(strings.NewReader(response))
	if err != nil || got.URL != "http://tlu.dl.delivery.mp.microsoft.com/package" || !bytes.Equal(got.Digest, payloadDigest[:]) {
		t.Fatalf("selected auxiliary file or borrowed its digest: %+v %v", got, err)
	}
	if _, err := parseLocations(strings.NewReader(locations("http://dl.delivery.mp.microsoft.com/auxiliary"))); err == nil {
		t.Fatal("accepted auxiliary location without game payload")
	}
}

func TestPackageVerifyChecksCompleteArchive(t *testing.T) {
	content := bytes.Repeat([]byte("Minecraft payload\x00"), 131072)
	filename := filepath.Join(t.TempDir(), "game.appx.download")
	if err := os.WriteFile(filename, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sha1Digest := sha1.Sum(content)
	sha256Digest := sha256.Sum256(content)
	for _, digest := range [][]byte{sha1Digest[:], sha256Digest[:]} {
		if err := (Package{Digest: digest}).Verify(filename); err != nil {
			t.Fatalf("valid archive: %v", err)
		}
	}
	// Keep the prefix valid and alter the final byte to detect partial hashing.
	content[len(content)-1] ^= 1
	if err := os.WriteFile(filename, content, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, digest := range [][]byte{sha1Digest[:], sha256Digest[:], nil, make([]byte, 16)} {
		if err := (Package{Digest: digest}).Verify(filename); err == nil || err.Error() != "ERR_UWP_INTEGRITY" {
			t.Fatalf("accepted corrupt archive/invalid digest (%d bytes): %v", len(digest), err)
		}
	}
	if err := (Package{Digest: sha256Digest[:]}).Verify(filename + ".missing"); !os.IsNotExist(err) {
		t.Fatalf("missing archive should preserve IO error: %v", err)
	}
}
