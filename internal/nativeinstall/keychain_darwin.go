//go:build darwin && cgo

package nativeinstall

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>
static OSStatus levi_cache_key(const char *account, int create, unsigned char *out) {
 CFStringRef service=CFSTR("org.levimc.levilauncher.native-cache");
 CFStringRef name=CFStringCreateWithCString(NULL,account,kCFStringEncodingUTF8);
 CFMutableDictionaryRef q=CFDictionaryCreateMutable(NULL,0,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 CFDictionarySetValue(q,kSecClass,kSecClassGenericPassword);CFDictionarySetValue(q,kSecAttrService,service);CFDictionarySetValue(q,kSecAttrAccount,name);
 CFDictionarySetValue(q,kSecReturnData,kCFBooleanTrue);CFDictionarySetValue(q,kSecMatchLimit,kSecMatchLimitOne);
 CFTypeRef result=NULL;OSStatus status=SecItemCopyMatching(q,&result);
 if(status==errSecItemNotFound && create) {
  unsigned char key[32];status=SecRandomCopyBytes(kSecRandomDefault,32,key);
  if(status==errSecSuccess) {
   CFDictionaryRemoveValue(q,kSecReturnData);CFDictionaryRemoveValue(q,kSecMatchLimit);
   CFDataRef data=CFDataCreate(NULL,key,32);CFDictionarySetValue(q,kSecValueData,data);
   status=SecItemAdd(q,NULL);CFRelease(data);
   if(status==errSecSuccess) memcpy(out,key,32);
   else if(status==errSecDuplicateItem) {
    CFDictionaryRemoveValue(q,kSecValueData);CFDictionarySetValue(q,kSecReturnData,kCFBooleanTrue);CFDictionarySetValue(q,kSecMatchLimit,kSecMatchLimitOne);status=SecItemCopyMatching(q,&result);
   }
  }
  memset(key,0,32);
 }
 if(status==errSecSuccess && result) {
  if(CFGetTypeID(result)!=CFDataGetTypeID()||CFDataGetLength((CFDataRef)result)!=32) status=errSecDecode;
  else memcpy(out,CFDataGetBytePtr((CFDataRef)result),32);
 }
 if(result) CFRelease(result);CFRelease(q);CFRelease(name);return status;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func systemProtectedKey(account string, create bool) ([]byte, error) {
	name := C.CString(account)
	defer C.free(unsafe.Pointer(name))
	key := make([]byte, 32)
	flag := C.int(0)
	if create {
		flag = 1
	}
	code := C.levi_cache_key(name, flag, (*C.uchar)(unsafe.Pointer(&key[0])))
	if code != 0 {
		clear(key)
		return nil, fmt.Errorf("macOS Keychain unavailable (status %d)", int(code))
	}
	return key, nil
}
