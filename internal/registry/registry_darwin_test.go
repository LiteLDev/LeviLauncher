package registry

import (
	"errors"
	"testing"
)

func TestGetAppxInfoOnMacOS(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	info, err := GetAppxInfo("MICROSOFT.MINECRAFTUWP")
	if info != nil || !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("GetAppxInfo returned (%v, %v), want unsupported", info, err)
	}
}
