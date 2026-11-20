//go:build darwin && cgo

package gameruntime

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
static int levi_visible_window(int pid){
 CFArrayRef windows=CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly|kCGWindowListExcludeDesktopElements,kCGNullWindowID);
 if(!windows)return 0;int found=0;
 for(CFIndex i=0;i<CFArrayGetCount(windows);i++){
  CFDictionaryRef d=(CFDictionaryRef)CFArrayGetValueAtIndex(windows,i);int owner=0,layer=0;
  CFNumberRef number=CFDictionaryGetValue(d,kCGWindowOwnerPID);if(!number)continue;CFNumberGetValue(number,kCFNumberIntType,&owner);if(owner!=pid)continue;
  number=CFDictionaryGetValue(d,kCGWindowLayer);if(number)CFNumberGetValue(number,kCFNumberIntType,&layer);
  CGRect bounds;CFDictionaryRef rect=CFDictionaryGetValue(d,kCGWindowBounds);
  if(layer==0&&rect&&CGRectMakeWithDictionaryRepresentation(rect,&bounds)&&bounds.size.width>64&&bounds.size.height>64){found=1;break;}
 }
 CFRelease(windows);return found;
}
*/
import "C"

func WindowVisible(pid int) bool { return C.levi_visible_window(C.int(pid)) != 0 }
