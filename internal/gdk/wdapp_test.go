package gdk

import (
	"encoding/xml"
	"testing"
)

func TestManifestDependenciesFromConfig(t *testing.T) {
	const publisher = "CN=Microsoft Corporation, O=Microsoft Corporation, L=Redmond, S=Washington, C=US"
	for _, tc := range []struct {
		name       string
		dependency string
		version    string
	}{
		{name: "legacy config"},
		{name: "runtime declared", dependency: "Microsoft.WindowsAppRuntime.1.8", version: "8000.770.947.0"},
		{name: "configured version preserved", dependency: "Microsoft.WindowsAppRuntime.1.8", version: "8000.999.1000.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configXML := `<Game><DesktopRegistration><DependencyList><KnownDependency Name="VC14" />`
			if tc.dependency != "" {
				configXML += `<Dependency Name="` + tc.dependency + `" MinVersion="` + tc.version + `" />`
			}
			configXML += `</DependencyList></DesktopRegistration></Game>`
			var cfg microsoftGameConfig
			if err := xml.Unmarshal([]byte(configXML), &cfg); err != nil {
				t.Fatal(err)
			}
			content, err := renderAppxManifest(&appxManifestData{PackageDependencies: packageDependenciesFromConfig(&cfg)})
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Dependencies []struct {
					Name       string `xml:"Name,attr"`
					MinVersion string `xml:"MinVersion,attr"`
					Publisher  string `xml:"Publisher,attr"`
				} `xml:"Dependencies>PackageDependency"`
			}
			if err := xml.Unmarshal(content, &manifest); err != nil {
				t.Fatal(err)
			}
			wantCount := 1
			if tc.dependency != "" {
				wantCount++
			}
			if len(manifest.Dependencies) != wantCount {
				t.Fatalf("got %d dependencies, want %d", len(manifest.Dependencies), wantCount)
			}
			vc := manifest.Dependencies[0]
			if vc.Name != "Microsoft.VCLibs.140.00.UWPDesktop" || vc.MinVersion != "14.0.33728.0" || vc.Publisher != publisher {
				t.Fatalf("unexpected VCLibs dependency: %+v", vc)
			}
			if tc.dependency != "" {
				runtime := manifest.Dependencies[1]
				if runtime.Name != tc.dependency || runtime.MinVersion != tc.version || runtime.Publisher != publisher {
					t.Fatalf("unexpected runtime dependency: %+v", runtime)
				}
			}
		})
	}
}
