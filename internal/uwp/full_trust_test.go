package uwp

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func assertFullTrustManifest(t *testing.T, data []byte) *etree.Document {
	t.Helper()
	// encoding/xml resolves namespace URIs independently of the rewriting library.
	var manifest struct {
		Application struct {
			Trust string `xml:"http://schemas.microsoft.com/appx/manifest/uap/windows10/10 TrustLevel,attr"`
		} `xml:"Applications>Application"`
	}
	if err := xml.Unmarshal(data, &manifest); err != nil || manifest.Application.Trust != "mediumIL" {
		t.Fatalf("full trust attribute: %+v, %v", manifest, err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		t.Fatal(err)
	}
	fullTrustCount, activationCount, previousOrder := 0, 0, 0
	for _, child := range doc.Root().SelectElement("Capabilities").ChildElements() {
		order := 0
		switch child.Tag {
		case "CustomCapability":
			order = 1
		case "DeviceCapability":
			order = 2
		}
		if order < previousOrder {
			t.Fatalf("capabilities violate schema order: %s", data)
		}
		previousOrder = order
		if child.Tag == "Capability" && child.NamespaceURI() == rescapNamespace && child.SelectAttrValue("Name", "") == "runFullTrust" {
			fullTrustCount++
		}
		if child.Tag == "CustomCapability" && child.NamespaceURI() == uap4Namespace && child.SelectAttrValue("Name", "") == coreAppActivation {
			activationCount++
		}
	}
	if fullTrustCount != 1 || activationCount != 1 {
		t.Fatalf("full trust capabilities: %d, %d", fullTrustCount, activationCount)
	}
	return doc
}

func TestFullTrustManifestPreservesPackageContent(t *testing.T) {
	manifest := strings.Replace(testManifest("neutral"), "<Package ", `<Package xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10" xmlns:custom="urn:existing" IgnorableNamespaces="uap custom" `, 1)
	manifest = strings.Replace(manifest, `Executable="Minecraft.Windows.exe"`, `Executable="Minecraft.Windows.exe" EntryPoint="Minecraft.App"`, 1)
	manifest = strings.Replace(manifest, "</Package>", `<Capabilities><Capability Name="internetClient"/><DeviceCapability Name="microphone"/><DeviceCapability Name="humaninterfacedevice"><Device Id="any"><Function Type="usage:0001 0002"/></Device></DeviceCapability></Capabilities><!-- retained --></Package>`, 1)
	modified, err := fullTrustManifest([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	doc := assertFullTrustManifest(t, modified)
	if got := doc.Root().FindElement("./Applications/Application").SelectAttrValue("EntryPoint", ""); got != "Minecraft.App" {
		t.Fatalf("changed UWP entry point: %s", got)
	}
	for _, retained := range []string{`xmlns:custom="urn:existing"`, `IgnorableNamespaces="uap custom uap10 rescap uap4"`, `<Capability Name="internetClient"/>`, `<DeviceCapability Name="microphone"/>`, `<Device Id="any"><Function Type="usage:0001 0002"/></Device>`, `<!-- retained -->`} {
		if !strings.Contains(string(modified), retained) {
			t.Fatalf("lost package content %s: %s", retained, modified)
		}
	}
	var originalIdentity, modifiedIdentity Manifest
	if err := xml.Unmarshal([]byte(manifest), &originalIdentity); err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(modified, &modifiedIdentity); err != nil || originalIdentity.Identity != modifiedIdentity.Identity {
		t.Fatalf("changed package identity: %+v, %v", modifiedIdentity, err)
	}
	second, err := fullTrustManifest(modified)
	if err != nil || !bytes.Equal(second, modified) {
		t.Fatalf("manifest rewrite is not idempotent: %s, %v", second, err)
	}
}

func TestFullTrustManifestHandlesMissingAndExistingCapabilities(t *testing.T) {
	for _, capabilityXML := range []string{
		"",
		`<Capabilities><rescap:Capability Name="other"/><rescap:Capability Name="runFullTrust"/><rescap:Capability Name="runFullTrust"/><uap4:CustomCapability Name="Other.Custom"/><uap4:CustomCapability Name="Microsoft.coreAppActivation_8wekyb3d8bbwe"/><uap4:CustomCapability Name="Microsoft.coreAppActivation_8wekyb3d8bbwe"/><DeviceCapability Name="microphone"/></Capabilities>`,
	} {
		manifest := strings.Replace(testManifest("neutral"), "<Package ", `<Package xmlns:rescap="`+rescapNamespace+`" xmlns:uap4="`+uap4Namespace+`" xmlns:uap10="`+uap10Namespace+`" `, 1)
		manifest = strings.Replace(manifest, `Id="App"`, `Id="App" uap10:TrustLevel="appContainer"`, 1)
		manifest = strings.Replace(manifest, "</Package>", capabilityXML+"</Package>", 1)
		modified, err := fullTrustManifest([]byte(manifest))
		if err != nil {
			t.Fatal(err)
		}
		assertFullTrustManifest(t, modified)
		if capabilityXML != "" && (!strings.Contains(string(modified), `Name="other"`) || !strings.Contains(string(modified), `Name="Other.Custom"`)) {
			t.Fatalf("removed unrelated capabilities: %s", modified)
		}
	}
}

func TestFullTrustManifestReplacesAliasedTrustLevel(t *testing.T) {
	manifest := strings.Replace(testManifest("neutral"), "<Package ", `<Package xmlns:trust="`+uap10Namespace+`" `, 1)
	manifest = strings.Replace(manifest, `Id="App"`, `Id="App" trust:TrustLevel="appContainer" trust:RuntimeBehavior="windowsApp"`, 1)
	modified, err := fullTrustManifest([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	assertFullTrustManifest(t, modified)
	if strings.Contains(string(modified), "trust:TrustLevel") || !strings.Contains(string(modified), `trust:RuntimeBehavior="windowsApp"`) {
		t.Fatalf("existing trust was duplicated or runtime changed: %s", modified)
	}
}

func TestFullTrustRegistrationRestoresFiles(t *testing.T) {
	for _, hasDescriptor := range []bool{false, true} {
		for _, registrationErr := range []error{nil, failure("ERR_UWP_DEPENDENCY", errors.New("missing dependency")), context.Canceled} {
			dir := t.TempDir()
			manifestPath := filepath.Join(dir, "AppxManifest.xml")
			descriptorPath := filepath.Join(dir, "CustomCapability.SCCD")
			original := []byte("\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n" + testManifest("neutral") + "\r\n")
			if err := os.WriteFile(manifestPath, original, 0644); err != nil {
				t.Fatal(err)
			}
			if hasDescriptor {
				if err := os.WriteFile(descriptorPath, []byte("original descriptor"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			err := withFullTrustManifest(dir, func() error {
				called = true
				data, err := os.ReadFile(manifestPath)
				if err != nil {
					t.Fatal(err)
				}
				assertFullTrustManifest(t, data)
				descriptor, err := os.ReadFile(descriptorPath)
				if err != nil || string(descriptor) != fullTrustDescriptor {
					t.Fatalf("missing activation descriptor: %s, %v", descriptor, err)
				}
				return registrationErr
			})
			if !called || !errors.Is(err, registrationErr) {
				t.Fatalf("registration result: %v, want %v", err, registrationErr)
			}
			if data, err := os.ReadFile(manifestPath); err != nil || !bytes.Equal(data, original) {
				t.Fatalf("manifest was not restored byte-for-byte: %s, %v", data, err)
			}
			if data, err := os.ReadFile(descriptorPath); hasDescriptor {
				if err != nil || string(data) != "original descriptor" {
					t.Fatalf("original descriptor was not restored: %s, %v", data, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("temporary descriptor was not removed: %v", err)
			}
		}
	}
}

func TestFullTrustPreparationFailureLeavesFilesUntouched(t *testing.T) {
	for _, invalidManifest := range []bool{false, true} {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "AppxManifest.xml")
		original := testManifest("neutral")
		if invalidManifest {
			original = "<Package>"
		} else if err := os.Mkdir(filepath.Join(dir, "CustomCapability.SCCD"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, []byte(original), 0644); err != nil {
			t.Fatal(err)
		}
		err := withFullTrustManifest(dir, func() error {
			t.Fatal("preparation failure must prevent deployment")
			return nil
		})
		if ErrorCode(err) != "ERR_UWP_PREPARE" {
			t.Fatalf("unexpected preparation result: %v", err)
		}
		if data, err := os.ReadFile(manifestPath); err != nil || string(data) != original {
			t.Fatalf("preparation changed original manifest: %s, %v", data, err)
		}
	}
}
