// Package instancereg registers instance folders with Windows as the
// installed Minecraft of their channel, and releases them, without a
// launcher window. The window's VersionService.RegisterVersionWithWdapp and
// UnregisterVersionByName call it too; results are "" or ERR_* codes.
package instancereg

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/gdk"
	"github.com/liteldev/LeviLauncher/internal/mcservice"
	"github.com/liteldev/LeviLauncher/internal/registry"
	"github.com/liteldev/LeviLauncher/internal/uwp"
	"github.com/liteldev/LeviLauncher/internal/versionlaunch"
	"github.com/liteldev/LeviLauncher/internal/versions"
)

// PackageName is the appx package that holds a channel's registration:
// preview has its own package, release and beta share the retail one.
func PackageName(isPreview bool) string {
	if isPreview {
		return "Microsoft.MinecraftWindowsBeta"
	}
	return "MICROSOFT.MINECRAFTUWP"
}

func normalizePath(p string) string {
	norm := strings.ToLower(filepath.Clean(strings.TrimSpace(p)))
	norm = strings.TrimPrefix(norm, `\\?\`)
	norm = strings.TrimPrefix(norm, `\??\`)
	return strings.TrimSuffix(norm, string(os.PathSeparator))
}

func instanceFolder(name string) (string, string) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", "ERR_TARGET_DIR_NOT_SPECIFIED"
	}
	if code := versionlaunch.ValidateLaunchName(n); code != "" {
		return "", code
	}
	vdir, err := apppath.InstancesDir()
	if err != nil || strings.TrimSpace(vdir) == "" {
		return "", "ERR_ACCESS_VERSIONS_DIR"
	}
	return filepath.Join(vdir, n), ""
}

// Register makes the instance the registered Minecraft of its channel. A GDK
// instance first removes whatever holds the channel (its data is kept); a
// UWP instance is registered in development mode over the current holder.
// The instance meta files are reconciled with Windows afterwards.
func Register(ctx context.Context, name string, isPreview bool) string {
	folder, code := instanceFolder(name)
	if code != "" {
		return code
	}
	defer mcservice.ReconcileRegisteredFlags()
	meta, _ := versions.ReadMeta(folder)
	if versions.DetectPackageType(folder, meta) == versions.PackageTypeUWP {
		if err := uwp.Register(ctx, folder); err != nil {
			log.Printf("instancereg.Register: UWP registration failed for %s: %v", name, err)
			return uwp.ErrorMessage(err)
		}
		return ""
	}
	if err := ctx.Err(); err != nil {
		return "ERR_CANCELED"
	}
	if msg := gdk.UnregisterIfExists(isPreview); msg != "" {
		log.Printf("instancereg.Register: pre-unregister returned %s for %s", msg, name)
		return msg
	}
	if msg := gdk.RegisterVersionFolder(folder); msg != "" {
		log.Printf("instancereg.Register: register failed for %s: %s", name, msg)
		return msg
	}
	if info, e := registry.GetAppxInfo(PackageName(isPreview)); e == nil && info != nil {
		flag := normalizePath(info.InstallLocation) == normalizePath(folder)
		if m, er := versions.ReadMeta(folder); er == nil {
			m.Registered = flag
			_ = versions.WriteMeta(folder, m)
		}
	}
	return ""
}

// Unregister releases the instance's registration, keeping the game data.
// An instance that is not registered is ERR_NOT_REGISTERED_THIS_VERSION for
// GDK and a no-op for UWP, as in the window.
func Unregister(ctx context.Context, name string) string {
	folder, code := instanceFolder(name)
	if code != "" {
		return code
	}
	defer mcservice.ReconcileRegisteredFlags()
	meta, _ := versions.ReadMeta(folder)
	if versions.DetectPackageType(folder, meta) == versions.PackageTypeUWP {
		if err := uwp.Unregister(ctx, folder); err != nil {
			log.Printf("instancereg.Unregister: UWP unregistration failed for %s: %v", name, err)
			return uwp.ErrorMessage(err)
		}
		return ""
	}
	if msg := gdk.UnregisterVersionFolder(folder); msg != "" {
		return msg
	}
	if m, er := versions.ReadMeta(folder); er == nil {
		m.Registered = false
		_ = versions.WriteMeta(folder, m)
	}
	return ""
}
