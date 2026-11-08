//go:build windows

package nativeinstall

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func rstFixture(t *testing.T, xml string, status int) rstResponse {
	t.Helper()
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatal(err)
	}
	return rstResponse{doc: doc, status: status}
}

const passportFault = `<S:Envelope xmlns:S="http://www.w3.org/2003/05/soap-envelope" xmlns:psf="http://schemas.microsoft.com/Passport/SoapServices/SOAPFault">
<S:Header><psf:pp><psf:reqstatus>0x80048821</psf:reqstatus></psf:pp></S:Header>
<S:Body><S:Fault>
<S:Code><S:Value>S:Sender</S:Value><S:Subcode><S:Value>wsse:FailedAuthentication</S:Value></S:Subcode></S:Code>
<S:Reason><S:Text xml:lang="en-US">Authentication Failure for member 02abcdef</S:Text></S:Reason>
<S:Detail><psf:error><psf:value>0x80048821</psf:value><psf:internalerror><psf:code>0x80041012</psf:code><psf:text>private server text</psf:text></psf:internalerror></psf:error></S:Detail>
</S:Fault></S:Body></S:Envelope>`

func TestRSTFaultKeepsOnlyCodes(t *testing.T) {
	f := parseRSTFault(rstFixture(t, passportFault, 400))
	if got, want := f.String(), "HTTP 400, wsse:FailedAuthentication, 0x80048821, 0x80041012"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if strings.Contains(f.String(), "02abcdef") || strings.Contains(f.String(), "private") {
		t.Fatal("server free text leaked into the fault summary")
	}
	if !f.deviceRejected() {
		t.Fatal("FailedAuthentication must count as a rejected device credential")
	}
}

func TestRSTFaultDropsMalformedValues(t *testing.T) {
	f := parseRSTFault(rstFixture(t, `<S:Envelope xmlns:S="http://www.w3.org/2003/05/soap-envelope"><S:Body><S:Fault>
<S:Code><S:Value>S:Receiver</S:Value><S:Subcode><S:Value>not a qname &lt;script&gt;</S:Value></S:Subcode></S:Code>
<S:Detail><error><value>bad</value></error></S:Detail></S:Fault></S:Body></S:Envelope>`, 500))
	if got := f.String(); got != "HTTP 500" {
		t.Fatalf("String() = %q", got)
	}
	if f.deviceRejected() {
		t.Fatal("an unidentified fault must not discard the device")
	}
}

func TestRSTFaultWithoutFaultElement(t *testing.T) {
	f := parseRSTFault(rstFixture(t, `<S:Envelope xmlns:S="http://www.w3.org/2003/05/soap-envelope"><S:Header><pp><reqstatus>0x00000000</reqstatus></pp></S:Header><S:Body/></S:Envelope>`, 200))
	if got := f.String(); got != "HTTP 200" {
		t.Fatalf("String() = %q", got)
	}
}

func TestRSTFaultServiceErrorIsNotDeviceRejection(t *testing.T) {
	f := parseRSTFault(rstFixture(t, `<S:Envelope xmlns:S="http://www.w3.org/2003/05/soap-envelope"><S:Body><S:Fault>
<S:Code><S:Value>S:Receiver</S:Value><S:Subcode><S:Value>psf:ServiceUnavailable</S:Value></S:Subcode></S:Code>
<S:Detail><error><value>0x80048800</value></error></S:Detail></S:Fault></S:Body></S:Envelope>`, 503))
	if f.deviceRejected() {
		t.Fatal("a service fault must not discard the device")
	}
	if got, want := f.String(), "HTTP 503, psf:ServiceUnavailable, 0x80048800"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
