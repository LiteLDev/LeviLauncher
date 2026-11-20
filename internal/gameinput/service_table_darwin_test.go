package gameinput

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisableGameInputServiceOnlyChangesItsStartType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ServiceInstall.idt")
	original := strings.Join([]string{
		"ServiceInstall\tName\tStartType\tDescription",
		"s72\ts255\ti4\tL255",
		"ServiceInstall\tServiceInstall",
		"UnrelatedService\tUnrelatedService\t3\tOther service",
		"ServiceInstaller\tGameInputRedistService\t3\tGameInput Redist Service",
		"",
	}, "\r\n")
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := disableGameInputService(path); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(original, "\tGameInputRedistService\t3\t", "\tGameInputRedistService\t4\t", 1)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("unexpected table contents %q", data)
	}
}
