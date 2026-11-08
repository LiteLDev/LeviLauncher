package app

import (
	"log"

	"github.com/liteldev/LeviLauncher/internal/instancereg"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// UpgradeInstance replaces the game build of an existing instance with the
// downloaded installer, keeping its data, mods and settings. backup zips the
// whole instance first.
func (a *Minecraft) UpgradeInstance(name string, installer string, gameVersion string, backup bool) mcservice.UpgradeResult {
	log.Printf("Minecraft.UpgradeInstance: name=%s installer=%s version=%s backup=%t", name, installer, gameVersion, backup)
	res := mcservice.UpgradeInstance(a.ctx, name, mcservice.UpgradeOptions{
		Installer:   installer,
		GameVersion: gameVersion,
		Backup:      backup,
		Unregister:  func() string { return instancereg.Unregister(a.ctx, name) },
	})
	// UWP instances register themselves again on launch; a GDK instance
	// launches unregistered from its executable, so restore the user's
	// registration here. A failed upgrade restored the old build, which was
	// registered too.
	if res.WasRegistered && res.PackageType == versions.PackageTypeGDK {
		if msg := instancereg.Register(a.ctx, name, res.IsPreview); msg != "" {
			log.Printf("Minecraft.UpgradeInstance: re-register %s failed: %s", name, msg)
		}
	}
	return res
}

// UpgradeInstanceDelta upgrades a UWP instance to the Windows Update package
// updateID, downloading only the blocks that differ from its current files.
func (a *Minecraft) UpgradeInstanceDelta(name string, updateID string, gameVersion string, backup bool) mcservice.UpgradeResult {
	log.Printf("Minecraft.UpgradeInstanceDelta: name=%s update=%s version=%s backup=%t", name, updateID, gameVersion, backup)
	return mcservice.UpgradeInstance(a.ctx, name, mcservice.UpgradeOptions{
		DeltaUpdateID: updateID,
		GameVersion:   gameVersion,
		Backup:        backup,
		Unregister:    func() string { return instancereg.Unregister(a.ctx, name) },
	})
}

// UpgradeTargetError reports why version cannot replace the instance's build,
// or "" when it can.
func (a *Minecraft) UpgradeTargetError(name string, version string, channel string, packageType string) string {
	return mcservice.UpgradeTargetError(name, version, channel, packageType)
}
