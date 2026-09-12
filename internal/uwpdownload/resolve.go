package uwpdownload

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/liteldev/LeviLauncher/internal/httpx"
)

const updateEndpoint = "https://fe3.delivery.mp.microsoft.com/ClientWebService/client.asmx"

type Package struct {
	URL    string
	Digest []byte
}

// Verify checks the complete archive against the digest delivered by the
// authenticated Windows Update response, including when its CDN URL is HTTP.
func (p Package) Verify(filename string) error {
	var h hash.Hash
	switch len(p.Digest) {
	case sha1.Size:
		h = sha1.New()
	case sha256.Size:
		h = sha256.New()
	default:
		return fmt.Errorf("ERR_UWP_INTEGRITY")
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if !bytes.Equal(h.Sum(nil), p.Digest) {
		return fmt.Errorf("ERR_UWP_INTEGRITY")
	}
	return nil
}

// The catalog contains Windows Update identities, not durable download URLs.
// Resolve on every download/retry because the returned CDN query is expiring.
func ResolveURL(ctx context.Context, updateID string) (string, error) {
	p, err := ResolvePackage(ctx, updateID)
	return p.URL, err
}

func ResolvePackage(ctx context.Context, updateID string) (Package, error) {
	if !updateIDPattern.MatchString(updateID) {
		return Package{}, fmt.Errorf("ERR_UWP_INVALID_VERSION")
	}
	return resolveURL(ctx, httpx.NewClient(30*time.Second), updateEndpoint, updateID)
}

func resolveURL(ctx context.Context, client *http.Client, endpoint, updateID string) (Package, error) {
	now := time.Now().UTC()
	body := fmt.Sprintf(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing" xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
<s:Header><a:Action s:mustUnderstand="1">http://www.microsoft.com/SoftwareDistribution/Server/ClientWebService/GetExtendedUpdateInfo2</a:Action><a:MessageID>urn:uuid:%s</a:MessageID><a:To>%s</a:To>
<o:Security s:mustUnderstand="1" xmlns:o="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"><u:Timestamp><u:Created>%s</u:Created><u:Expires>%s</u:Expires></u:Timestamp><t:WindowsUpdateTicketsToken xmlns:t="http://schemas.microsoft.com/msus/2014/10/WindowsUpdateAuthorization" u:id="ClientMSA"><t:TicketType Name="AAD" Version="1.0" Policy="MBI_SSL"/></t:WindowsUpdateTicketsToken></o:Security></s:Header>
<s:Body><GetExtendedUpdateInfo2 xmlns="http://www.microsoft.com/SoftwareDistribution/Server/ClientWebService"><protocolVersion>1.81</protocolVersion><updateIDs><UpdateIdentity><UpdateID>%s</UpdateID><RevisionNumber>1</RevisionNumber></UpdateIdentity></updateIDs><infoTypes><XmlUpdateFragmentType>FileUrl</XmlUpdateFragmentType></infoTypes></GetExtendedUpdateInfo2></s:Body></s:Envelope>`, updateID, updateEndpoint, now.Format(time.RFC3339), now.Add(5*time.Minute).Format(time.RFC3339), updateID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return Package{}, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("User-Agent", "Windows-Update-Agent/10.0.19041.2546 Client-Protocol/1.81")
	resp, err := client.Do(req)
	if err != nil {
		return Package{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Package{}, fmt.Errorf("Windows Update HTTP %d", resp.StatusCode)
	}
	return parseLocations(resp.Body)
}

func parseLocations(r io.Reader) (Package, error) {
	var response struct {
		Locations []struct {
			URL    string `xml:"Url"`
			Digest string `xml:"FileDigest"`
		} `xml:"Body>GetExtendedUpdateInfo2Response>GetExtendedUpdateInfo2Result>FileLocations>FileLocation"`
	}
	if err := xml.NewDecoder(io.LimitReader(r, 4<<20)).Decode(&response); err != nil {
		return Package{}, fmt.Errorf("decode Windows Update locations: %w", err)
	}
	// The tlu location is the game payload. Other locations in the same
	// response can be auxiliary files, so preserve this preference explicitly.
	for _, location := range response.Locations {
		u, err := url.Parse(strings.TrimSpace(location.URL))
		if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host != "tlu.dl.delivery.mp.microsoft.com" {
			continue
		}
		digest, err := base64.StdEncoding.DecodeString(strings.TrimSpace(location.Digest))
		if err != nil || (len(digest) != sha1.Size && len(digest) != sha256.Size) {
			continue
		}
		// Keep Microsoft's scheme and signed query. Regional CDN endpoints
		// may serve HTTP only; TLS verification remains enabled for WSUS.
		return Package{URL: u.String(), Digest: digest}, nil
	}
	return Package{}, fmt.Errorf("ERR_UWP_DOWNLOAD_URL")
}
