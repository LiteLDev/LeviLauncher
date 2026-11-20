//go:build darwin

package leviloader

import (
	"context"
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

// PatchAndActivateForWine retains an instance's verified compatible loader,
// when supplied in .levilauncher/wine-loader.dll. Windows always uses the
// bundled loader through PatchAndActivate instead.
func PatchAndActivateForWine(ctx context.Context, dir string) (bool, error) {
	path := filepath.Join(apppath.InstanceStateDir(dir), "wine-loader.dll")
	loader, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return PatchAndActivate(ctx, dir)
	}
	if err != nil {
		return false, err
	}
	image, err := pe.Open(path)
	if err != nil {
		return false, fmt.Errorf("invalid Wine instance loader: %w", err)
	}
	supported := image.Machine == pe.IMAGE_FILE_MACHINE_AMD64 && image.Characteristics&pe.IMAGE_FILE_DLL != 0
	image.Close()
	if !supported {
		return false, fmt.Errorf("Wine instance loader must be a Windows x64 DLL")
	}
	added, err := patchAndActivate(ctx, dir, loader)
	if err != nil {
		return added, err
	}
	// Historical compatible loaders read version.json. Keep instance.json
	// authoritative: publishing a newer legacy file would trigger migration
	// on a concurrent instance-list refresh and remove the runtime input.
	current := filepath.Join(apppath.InstanceStateDir(dir), apppath.InstanceMetaFileName)
	info, err := os.Stat(current)
	if err != nil {
		return added, err
	}
	data, err := os.ReadFile(current)
	if err != nil {
		return added, err
	}
	temp, err := os.CreateTemp(apppath.InstanceStateDir(dir), "wine-meta-*.json")
	if err != nil {
		return added, err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if _, err = temp.Write(data); err != nil {
		return added, err
	}
	if err = temp.Close(); err != nil {
		return added, err
	}
	if err = os.Chtimes(temp.Name(), info.ModTime(), info.ModTime()); err != nil {
		return added, err
	}
	return added, os.Rename(temp.Name(), filepath.Join(dir, apppath.LegacyInstanceMetaFileName))
}
