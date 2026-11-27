//go:build darwin && cgo

package gameruntime

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
#include <IOKit/graphics/IOGraphicsTypes.h>
#include <math.h>
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
static double levi_main_display_retina_oversample_ratio(void){
 CGDirectDisplayID did=CGMainDisplayID();
 if(!did)return 1.0;
 CGDisplayModeRef cur=CGDisplayCopyDisplayMode(did);
 if(!cur)return 1.0;
 double lw=(double)CGDisplayModeGetWidth(cur);
 double lh=(double)CGDisplayModeGetHeight(cur);
 double bw=(double)CGDisplayModeGetPixelWidth(cur);
 double bh=(double)CGDisplayModeGetPixelHeight(cur);
 CGDisplayModeRelease(cur);
 if(lw<=0.0||lh<=0.0||bw<=0.0||bh<=0.0)return 1.0;
 double backingScale=fmin(bw/lw,bh/lh);
 if(backingScale<=1.01)return 1.0;
 CFArrayRef modes=CGDisplayCopyAllDisplayModes(did,NULL);
 if(!modes)return 1.0;
 double panelScale=0.0;
 for(CFIndex i=0;i<CFArrayGetCount(modes);i++){
  CGDisplayModeRef mode=(CGDisplayModeRef)CFArrayGetValueAtIndex(modes,i);
  if(!(CGDisplayModeGetIOFlags(mode)&kDisplayModeNativeFlag))continue;
  size_t pw=CGDisplayModeGetPixelWidth(mode);
  size_t ph=CGDisplayModeGetPixelHeight(mode);
  if(!pw||!ph||pw!=CGDisplayModeGetWidth(mode)||ph!=CGDisplayModeGetHeight(mode))continue;
  double candidate=fmin((double)pw/lw,(double)ph/lh);
  panelScale=panelScale>0.0?fmin(panelScale,candidate):candidate;
 }
 CFRelease(modes);
 if(panelScale<=0.0||panelScale>=backingScale)return 1.0;
 return backingScale/panelScale;
}
*/
import "C"

func WindowVisible(pid int) bool { return C.levi_visible_window(C.int(pid)) != 0 }

func mainDisplayRetinaOversampleRatio() float64 {
	return float64(C.levi_main_display_retina_oversample_ratio())
}
