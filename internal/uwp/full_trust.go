package uwp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/beevik/etree"
)

const (
	uap10Namespace    = "http://schemas.microsoft.com/appx/manifest/uap/windows10/10"
	uap4Namespace     = "http://schemas.microsoft.com/appx/manifest/uap/windows10/4"
	rescapNamespace   = "http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities"
	coreAppActivation = "Microsoft.coreAppActivation_8wekyb3d8bbwe"
	// Match levilauncher-source's development descriptor for current-user
	// full trust. This does not request administrator elevation.
	fullTrustDescriptor = `<?xml version="1.0" encoding="utf-8"?>
<CustomCapabilityDescriptor xmlns="http://schemas.microsoft.com/appx/2018/sccd" xmlns:s="http://schemas.microsoft.com/appx/2018/sccd">
  <CustomCapabilities>
    <CustomCapability Name="Microsoft.coreAppActivation_8wekyb3d8bbwe"/>
  </CustomCapabilities>
  <AuthorizedEntities AllowAny="true"/>
  <Catalog>FFFF</Catalog>
</CustomCapabilityDescriptor>
`
)

func fullTrustManifest(data []byte) ([]byte, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, err
	}
	pkg := doc.Root()
	if pkg == nil || pkg.Tag != "Package" {
		return nil, fmt.Errorf("missing Package element")
	}
	app := pkg.FindElement("./Applications/Application")
	if app == nil {
		return nil, fmt.Errorf("missing Application element")
	}
	ignorable := strings.Fields(pkg.SelectAttrValue("IgnorableNamespaces", ""))
	for _, ns := range []struct{ prefix, uri string }{
		{"uap10", uap10Namespace}, {"rescap", rescapNamespace}, {"uap4", uap4Namespace},
	} {
		if attr := pkg.SelectAttr("xmlns:" + ns.prefix); attr != nil && attr.Value != ns.uri {
			return nil, fmt.Errorf("unexpected namespace for %s", ns.prefix)
		}
		pkg.CreateAttr("xmlns:"+ns.prefix, ns.uri)
		if !slices.Contains(ignorable, ns.prefix) {
			ignorable = append(ignorable, ns.prefix)
		}
	}
	pkg.CreateAttr("IgnorableNamespaces", strings.Join(ignorable, " "))
	// Preserve the UWP entry point and remove any existing trust attribute,
	// including one declared through a different prefix for the same namespace.
	for _, attr := range slices.Clone(app.Attr) {
		if attr.Key == "TrustLevel" && attr.NamespaceURI() == uap10Namespace {
			app.RemoveAttr(attr.FullKey())
		}
	}
	app.CreateAttr("uap10:TrustLevel", "mediumIL")
	capabilities := pkg.SelectElement("Capabilities")
	if capabilities == nil {
		capabilities = pkg.CreateElement("Capabilities")
	}
	// Keep full DeviceCapability nodes and insert capabilities in schema order:
	// Capability, CustomCapability, DeviceCapability.
	for _, capability := range capabilities.ChildElements() {
		name := capability.SelectAttrValue("Name", "")
		if (capability.Tag == "Capability" && capability.NamespaceURI() == rescapNamespace && name == "runFullTrust") ||
			(capability.Tag == "CustomCapability" && capability.NamespaceURI() == uap4Namespace && name == coreAppActivation) {
			capabilities.RemoveChild(capability)
		}
	}
	fullTrust := etree.NewElement("rescap:Capability")
	fullTrust.CreateAttr("Name", "runFullTrust")
	activation := etree.NewElement("uap4:CustomCapability")
	activation.CreateAttr("Name", coreAppActivation)
	var firstCustomOrDevice, firstDevice *etree.Element
	for _, capability := range capabilities.ChildElements() {
		if firstCustomOrDevice == nil && (capability.Tag == "CustomCapability" || capability.Tag == "DeviceCapability") {
			firstCustomOrDevice = capability
		}
		if firstDevice == nil && capability.Tag == "DeviceCapability" {
			firstDevice = capability
		}
	}
	if firstCustomOrDevice == nil {
		capabilities.AddChild(fullTrust)
	} else {
		capabilities.InsertChild(firstCustomOrDevice, fullTrust)
	}
	if firstDevice == nil {
		capabilities.AddChild(activation)
	} else {
		capabilities.InsertChild(firstDevice, activation)
	}
	return doc.WriteToBytes()
}

// withFullTrustManifest restores the original files byte-for-byte on success,
// deployment failure or cancellation, including an existing custom descriptor.
func withFullTrustManifest(dir string, register func() error) (result error) {
	manifestPath := filepath.Join(dir, "AppxManifest.xml")
	descriptorPath := filepath.Join(dir, "CustomCapability.SCCD")
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return failure("ERR_UWP_PREPARE", err)
	}
	if !manifestInfo.Mode().IsRegular() {
		return failure("ERR_UWP_PREPARE", fmt.Errorf("package manifest is not a regular file"))
	}
	originalManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return failure("ERR_UWP_PREPARE", err)
	}
	modifiedManifest, err := fullTrustManifest(originalManifest)
	if err != nil {
		return failure("ERR_UWP_PREPARE", err)
	}
	var originalDescriptor []byte
	descriptorInfo, err := os.Lstat(descriptorPath)
	if err == nil {
		if !descriptorInfo.Mode().IsRegular() {
			return failure("ERR_UWP_PREPARE", fmt.Errorf("custom capability descriptor is not a regular file"))
		}
		originalDescriptor, err = os.ReadFile(descriptorPath)
		if err != nil {
			return failure("ERR_UWP_PREPARE", err)
		}
	} else if !os.IsNotExist(err) {
		return failure("ERR_UWP_PREPARE", err)
	}
	defer func() {
		manifestErr := os.WriteFile(manifestPath, originalManifest, manifestInfo.Mode().Perm())
		var descriptorErr error
		if descriptorInfo != nil {
			descriptorErr = os.WriteFile(descriptorPath, originalDescriptor, descriptorInfo.Mode().Perm())
		} else {
			descriptorErr = os.Remove(descriptorPath)
			if os.IsNotExist(descriptorErr) {
				descriptorErr = nil
			}
		}
		if err := errors.Join(manifestErr, descriptorErr); err != nil {
			result = failure("ERR_UWP_PREPARE", errors.Join(result, fmt.Errorf("restore registration files: %w", err)))
		}
	}()
	if err := os.WriteFile(descriptorPath, []byte(fullTrustDescriptor), 0644); err != nil {
		return failure("ERR_UWP_PREPARE", err)
	}
	if err := os.WriteFile(manifestPath, modifiedManifest, manifestInfo.Mode().Perm()); err != nil {
		return failure("ERR_UWP_PREPARE", err)
	}
	return register()
}
