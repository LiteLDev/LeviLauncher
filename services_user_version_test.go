package main

import (
	"fmt"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/versions"
)

func TestBuildVersionMenuDetailsPreservesInputOrderAndValues(t *testing.T) {
	metas := []versions.VersionMeta{
		{Name: "Preview", Registered: true},
		{Name: "Release", Registered: false},
	}

	got := buildVersionMenuDetails(
		metas,
		func(name string) string {
			return fmt.Sprintf("logo:%s", name)
		},
		func(name string) bool {
			return name == "Release"
		},
	)

	if len(got) != len(metas) {
		t.Fatalf("details length = %d, want %d", len(got), len(metas))
	}
	if got[0].Name != "Preview" || !got[0].Registered {
		t.Fatalf("first detail = %#v", got[0])
	}
	if got[0].LogoDataURL != "logo:Preview" || got[0].LeviLaminaInstalled {
		t.Fatalf("first detail enrichment = %#v", got[0])
	}
	if got[1].Name != "Release" || got[1].Registered {
		t.Fatalf("second detail = %#v", got[1])
	}
	if got[1].LogoDataURL != "logo:Release" || !got[1].LeviLaminaInstalled {
		t.Fatalf("second detail enrichment = %#v", got[1])
	}
}

func TestBuildVersionMenuDetailsHandlesEmptyInput(t *testing.T) {
	got := buildVersionMenuDetails(nil, nil, nil)
	if got == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("details length = %d, want 0", len(got))
	}
}

func TestBuildVersionMenuDetailsPreservesNameAndUsesNormalizedLookup(t *testing.T) {
	metaName := " Preview "
	var logoLookup string
	var modLookup string

	got := buildVersionMenuDetails(
		[]versions.VersionMeta{{Name: metaName}},
		func(name string) string {
			logoLookup = name
			return "logo"
		},
		func(name string) bool {
			modLookup = name
			return true
		},
	)

	if len(got) != 1 || got[0].Name != metaName {
		t.Fatalf("details = %#v, want original name %q", got, metaName)
	}
	if logoLookup != "Preview" || modLookup != "Preview" {
		t.Fatalf(
			"lookup names = (%q, %q), want normalized Preview",
			logoLookup,
			modLookup,
		)
	}
}
