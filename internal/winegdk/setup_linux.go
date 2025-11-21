//go:build linux

package winegdk

import (
    "context"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "github.com/wailsapp/wails/v3/pkg/application"
    "github.com/liteldev/LeviLauncher/internal/utils"
    "os/ioutil"
)

const (
    EventSetupStatus   = "winegdk.setup.status"
    EventSetupProgress = "winegdk.setup.progress"
    EventSetupError    = "winegdk.setup.error"
    EventSetupDone     = "winegdk.setup.done"
)

func Setup(ctx context.Context) string {
    base := utils.BaseRoot()
    if strings.TrimSpace(base) == "" { application.Get().Event.Emit(EventSetupError, "ERR_BASE_ROOT"); return "ERR_BASE_ROOT" }
    application.Get().Event.Emit(EventSetupStatus, "start")
    id := func() string {
        b, err := ioutil.ReadFile("/etc/os-release")
        if err != nil { return "" }
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
        _ = cmd.Run()
    } else {
        application.Get().Event.Emit(EventSetupStatus, "deps_warning_other")
    }
    wg := filepath.Join(base, "WineGDK")
    if _, err := os.Stat(wg); err != nil {
        application.Get().Event.Emit(EventSetupStatus, "cloning")
        cmd := exec.Command("git", "clone", "https://github.com/Weather-OS/WineGDK.git", wg)
        if err := cmd.Run(); err != nil { application.Get().Event.Emit(EventSetupError, "ERR_GIT_CLONE"); return "ERR_GIT_CLONE" }
    }
    bd := filepath.Join(base, "build")
    _ = os.MkdirAll(bd, 0755)
    application.Get().Event.Emit(EventSetupStatus, "configuring")
    cfg := exec.Command("bash", "-c", "cd '"+bd+"' && '../WineGDK/configure' --enable-win64")
    if err := cfg.Run(); err != nil { application.Get().Event.Emit(EventSetupError, "ERR_CONFIGURE"); return "ERR_CONFIGURE" }
    application.Get().Event.Emit(EventSetupStatus, "compiling")
    mk := exec.Command("bash", "-c", "cd '"+bd+"' && make -j$(nproc)")
    if err := mk.Run(); err != nil { application.Get().Event.Emit(EventSetupError, "ERR_MAKE"); return "ERR_MAKE" }
    pf := filepath.Join(base, "prefix")
    _ = os.MkdirAll(pf, 0755)
    application.Get().Event.Emit(EventSetupStatus, "winetricks")
    wt := exec.Command("bash", "-c", "WINEPREFIX='"+pf+"' winetricks vkd3d dxvk dxvk_nvapi0061")
    if err := wt.Run(); err != nil { application.Get().Event.Emit(EventSetupError, "ERR_WINETRICKS"); return "ERR_WINETRICKS" }
    application.Get().Event.Emit(EventSetupDone, struct{}{})
    return ""
}