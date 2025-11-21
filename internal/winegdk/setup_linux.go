//go:build linux

package winegdk

import (
    "bufio"
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/liteldev/LeviLauncher/internal/utils"
    "github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventSetupStatus   = "winegdk.setup.status"
	EventSetupProgress = "winegdk.setup.progress"
	EventSetupError    = "winegdk.setup.error"
	EventSetupDone     = "winegdk.setup.done"
)

func Setup(ctx context.Context) string {
    base := utils.BaseRoot()
	if strings.TrimSpace(base) == "" {
		application.Get().Event.Emit(EventSetupError, "ERR_BASE_ROOT")
		return "ERR_BASE_ROOT"
	}
    application.Get().Event.Emit(EventSetupStatus, "start")
    if _, err := exec.LookPath("wine"); err != nil {
        application.Get().Event.Emit(EventSetupStatus, "deps_warning_wine_missing")
    }
	id := func() string {
		b, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return ""
		}
		for _, l := range strings.Split(string(b), "\n") {
			s := strings.TrimSpace(l)
			if strings.HasPrefix(s, "ID=") {
				v := strings.TrimPrefix(s, "ID=")
				v = strings.Trim(v, "\"'")
				return strings.ToLower(strings.TrimSpace(v))
			}
		}
		return ""
	}()
	if id == "arch" {
		application.Get().Event.Emit(EventSetupStatus, "deps_warning_arch")
		pkgs := "mingw-w64-gcc base-devel git gcc multilib-devel winetricks wine vulkan-icd-loader lib32-vulkan-icd-loader libx11 lib32-libx11 freetype2 lib32-freetype2 mesa lib32-mesa glu lib32-glu alsa-lib lib32-alsa-lib libxrandr lib32-libxrandr libxi lib32-libxi libxext lib32-libxext libxrender lib32-libxrender libxcursor lib32-libxcursor libxinerama lib32-libxinerama libxcomposite lib32-libxcomposite libxfixes lib32-libxfixes libpng lib32-libpng libjpeg-turbo lib32-libjpeg-turbo libtiff lib32-libtiff openal lib32-openal mpg123 lib32-mpg123 sdl2 lib32-sdl2 libxml2 lib32-libxml2 libldap lib32-libldap vulkan-headers cups"
		application.Get().Event.Emit(EventSetupStatus, "deps_install_arch")
        cmd := exec.Command("bash", "-c", "sudo pacman -S --needed --noconfirm "+pkgs)
        _ = streamCmd(cmd, "deps")
	} else {
		application.Get().Event.Emit(EventSetupStatus, "deps_warning_other")
	}
    wg := filepath.Join(base, "WineGDK")
    if _, err := os.Stat(wg); err != nil {
        application.Get().Event.Emit(EventSetupStatus, "cloning")
        cmd := exec.Command("git", "clone", "https://github.com/Weather-OS/WineGDK.git", wg)
        if err := streamCmd(cmd, "clone"); err != nil {
            application.Get().Event.Emit(EventSetupError, "ERR_GIT_CLONE")
            return "ERR_GIT_CLONE"
        }
    } else {
        application.Get().Event.Emit(EventSetupStatus, "updating")
        _ = streamCmd(exec.Command("bash", "-c", "cd '"+wg+"' && git remote update"), "update")
    }
    needBuild := false
    if _, err := os.Stat(wg); err == nil {
        chk := exec.Command("bash", "-c", "cd '"+wg+"' && if [ \"$(git rev-parse HEAD)\" = \"$(git rev-parse @{u})\" ]; then echo up_to_date; else echo needs_update; fi")
        out, _ := chk.CombinedOutput()
        s := strings.ToLower(strings.TrimSpace(string(out)))
        if s == "needs_update" {
            application.Get().Event.Emit(EventSetupStatus, "pulling")
            if er := streamCmd(exec.Command("bash", "-c", "cd '"+wg+"' && git pull --rebase"), "pull"); er != nil {
                application.Get().Event.Emit(EventSetupError, "ERR_GIT_PULL")
                return "ERR_GIT_PULL"
            }
            needBuild = true
        }
    } else {
        needBuild = true
    }
    bd := filepath.Join(wg, "build")
    _ = os.MkdirAll(bd, 0755)
    if needBuild {
        application.Get().Event.Emit(EventSetupStatus, "configuring")
        cfg := exec.Command("bash", "-c", "cd '"+bd+"' && ../configure --enable-win64")
        if err := streamCmd(cfg, "configure"); err != nil {
            application.Get().Event.Emit(EventSetupError, "ERR_CONFIGURE")
            return "ERR_CONFIGURE"
        }
        application.Get().Event.Emit(EventSetupStatus, "compiling")
        mk := exec.Command("bash", "-c", "cd '"+bd+"' && make -j$(nproc)")
        if err := streamCmd(mk, "make"); err != nil {
            application.Get().Event.Emit(EventSetupError, "ERR_MAKE")
            return "ERR_MAKE"
        }
    } else {
        application.Get().Event.Emit(EventSetupStatus, "skip_build")
    }
	pf := filepath.Join(base, "prefix")
	_ = os.MkdirAll(pf, 0755)
	application.Get().Event.Emit(EventSetupStatus, "winetricks")
    wt := exec.Command("bash", "-c", "WINEPREFIX='"+pf+"' winetricks vkd3d dxvk dxvk_nvapi0061")
    if err := streamCmd(wt, "winetricks"); err != nil {
        application.Get().Event.Emit(EventSetupError, "ERR_WINETRICKS")
        return "ERR_WINETRICKS"
    }
	application.Get().Event.Emit(EventSetupDone, struct{}{})
	return ""
}

func streamCmd(cmd *exec.Cmd, phase string) error {
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()
    if err := cmd.Start(); err != nil {
        return err
    }
    go func() {
        sc := bufio.NewScanner(stdout)
        for sc.Scan() {
            application.Get().Event.Emit(EventSetupProgress, map[string]interface{}{"phase": phase, "stream": "stdout", "line": sc.Text()})
        }
    }()
    go func() {
        sc := bufio.NewScanner(stderr)
        for sc.Scan() {
            application.Get().Event.Emit(EventSetupProgress, map[string]interface{}{"phase": phase, "stream": "stderr", "line": sc.Text()})
        }
    }()
    return cmd.Wait()
}
