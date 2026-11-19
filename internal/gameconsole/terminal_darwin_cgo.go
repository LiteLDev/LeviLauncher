//go:build darwin && cgo

package gameconsole

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <CoreServices/CoreServices.h>
#include <stdlib.h>

static char *levi_copy_cfstring_utf8(CFStringRef str) {
	if (!str) return NULL;
	CFIndex maxLen = CFStringGetMaximumSizeForEncoding(CFStringGetLength(str), kCFStringEncodingUTF8) + 1;
	char *buf = (char *)malloc((size_t)maxLen);
	if (!buf) return NULL;
	if (!CFStringGetCString(str, buf, maxLen, kCFStringEncodingUTF8)) {
		free(buf);
		return NULL;
	}
	return buf;
}

static char *levi_default_bundle_for_file(const char *path) {
	if (!path || !path[0]) return NULL;
	CFURLRef fileURL = CFURLCreateFromFileSystemRepresentation(
		kCFAllocatorDefault, (const UInt8 *)path, (CFIndex)strlen(path), false);
	if (!fileURL) return NULL;
	CFURLRef appURL = LSCopyDefaultApplicationURLForURL(fileURL, kLSRolesAll, NULL);
	CFRelease(fileURL);
	if (!appURL) return NULL;
	CFBundleRef bundle = CFBundleCreate(kCFAllocatorDefault, appURL);
	CFRelease(appURL);
	if (!bundle) return NULL;
	CFStringRef bid = CFBundleGetIdentifier(bundle);
	char *result = levi_copy_cfstring_utf8(bid);
	CFRelease(bundle);
	return result;
}

static char *levi_default_bundle_for_uti(const char *uti) {
	if (!uti || !uti[0]) return NULL;
	CFStringRef cfUTI = CFStringCreateWithCString(kCFAllocatorDefault, uti, kCFStringEncodingUTF8);
	if (!cfUTI) return NULL;
	CFStringRef bid = LSCopyDefaultRoleHandlerForContentType(cfUTI, kLSRolesAll);
	CFRelease(cfUTI);
	if (!bid) return NULL;
	char *result = levi_copy_cfstring_utf8(bid);
	CFRelease(bid);
	return result;
}
*/
import "C"
import "unsafe"

func defaultBundleIDForFile(path string) string {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	res := C.levi_default_bundle_for_file(cpath)
	if res == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(res))
	return C.GoString(res)
}

func defaultBundleIDForUTI(uti string) string {
	cuti := C.CString(uti)
	defer C.free(unsafe.Pointer(cuti))
	res := C.levi_default_bundle_for_uti(cuti)
	if res == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(res))
	return C.GoString(res)
}
