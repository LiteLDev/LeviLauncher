package nativeinstall

import (
	"os"

	"golang.org/x/sys/windows"
)

func secureCache(dir string) {
	must(os.MkdirAll(dir, 0700))
	info, e := os.Lstat(dir)
	must(e)
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		fail("credential cache is not a real directory")
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	must(e)
	sd, e := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	must(e)
	path, e := windows.UTF16PtrFromString(dir)
	must(e)
	r, _, e := windows.NewLazySystemDLL("advapi32.dll").NewProc("SetFileSecurityW").Call(ptr(path), 0x80000004, ptr(sd))
	if r == 0 {
		must(e)
	}
}
