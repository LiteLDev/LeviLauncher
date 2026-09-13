package versions

import "testing"

func TestMinecraftExecutableNames(t *testing.T) {
	for _, name := range []string{"Minecraft.Windows.exe", "minecraft.win10.dx11.EXE"} {
		if !IsMinecraftExecutable(name) {
			t.Errorf("missing Minecraft process %q", name)
		}
	}
	for _, name := range []string{"Minecraft.Windows.exe.bak", "Other.exe", "Minecraft.exe"} {
		if IsMinecraftExecutable(name) {
			t.Errorf("accepted unrelated process %q", name)
		}
	}
}
