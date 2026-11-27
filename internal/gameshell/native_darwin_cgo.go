//go:build darwin && cgo

package gameshell

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UTType.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	int cancelled;
	int count;
	char **paths;
} LeviDialogResultC;

static void levi_free_dialog_result(LeviDialogResultC *res) {
	if (!res) return;
	if (res->paths) {
		for (int i = 0; i < res->count; ++i) {
			free(res->paths[i]);
		}
		free(res->paths);
		res->paths = NULL;
	}
	res->count = 0;
}

static int levi_activate_app_for_panel(void) {
	NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
	NSRunningApplication *selfApp = [NSRunningApplication currentApplication];
	int prevPID = 0;
	if (front != nil && selfApp != nil && front.processIdentifier != selfApp.processIdentifier) {
		prevPID = (int)front.processIdentifier;
	}
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
	[NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
	if (@available(macOS 14.0, *)) {
		[NSApp activate];
	}
	return prevPID;
}

static void levi_restore_app_after_panel(int pid) {
	if (pid <= 0) return;
	NSRunningApplication *target = [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pid];
	if (target != nil && !target.terminated) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
		[target activateWithOptions:NSApplicationActivateIgnoringOtherApps];
#pragma clang diagnostic pop
	}
}

static void levi_apply_allowed_extensions(NSSavePanel *panel, const char *extsCSV, int allowOther) {
	if (allowOther || !extsCSV || !extsCSV[0]) {
		[panel setAllowsOtherFileTypes:YES];
		return;
	}
	NSString *csv = [NSString stringWithUTF8String:extsCSV];
	if (!csv || [csv length] == 0) {
		[panel setAllowsOtherFileTypes:YES];
		return;
	}
	NSArray<NSString *> *parts = [csv componentsSeparatedByString:@";"];
	if (@available(macOS 11.0, *)) {
		NSMutableArray<UTType *> *types = [NSMutableArray array];
		for (NSString *ext in parts) {
			if ([ext length] == 0) continue;
			UTType *t = [UTType typeWithFilenameExtension:ext];
			if (t != nil) {
				[types addObject:t];
			}
		}
		if ([types count] > 0) {
			[panel setAllowedContentTypes:types];
			[panel setAllowsOtherFileTypes:NO];
			return;
		}
	}
	[panel setAllowsOtherFileTypes:YES];
}

static NSSavePanel *g_active_panel = nil;

@interface LeviPanelFocusHelper : NSObject
+ (void)focusActivePanel;
@end

@implementation LeviPanelFocusHelper
+ (void)focusActivePanel {
	if (g_active_panel != nil) {
		levi_activate_app_for_panel();
		[g_active_panel setLevel:NSModalPanelWindowLevel];
		[g_active_panel orderFrontRegardless];
		[g_active_panel makeKeyAndOrderFront:nil];
	}
}
@end

static void levi_focus_active_panel(void) {
	@autoreleasepool {
		[LeviPanelFocusHelper performSelectorOnMainThread:@selector(focusActivePanel)
		                                       withObject:nil
		                                    waitUntilDone:NO
		                                            modes:@[NSRunLoopCommonModes, NSModalPanelRunLoopMode]];
	}
}

static LeviDialogResultC levi_run_native_panel(
	int kind, // 0=open, 1=save, 2=folder
	const char *title,
	const char *initialDir,
	const char *defaultName,
	const char *extsCSV,
	int allowOther,
	int multi
) {
	LeviDialogResultC res;
	res.cancelled = 1;
	res.count = 0;
	res.paths = NULL;

	@autoreleasepool {
		int prevPID = levi_activate_app_for_panel();
		if (kind == 1) {
			NSSavePanel *panel = [NSSavePanel savePanel];
			[panel setCanCreateDirectories:YES];
			[panel setAllowsOtherFileTypes:YES];
			if (title && title[0]) {
				NSString *t = [NSString stringWithUTF8String:title];
				if (t) {
					[panel setTitle:t];
					[panel setMessage:t];
				}
			}
			if (initialDir && initialDir[0]) {
				NSString *d = [NSString stringWithUTF8String:initialDir];
				if (d) [panel setDirectoryURL:[NSURL fileURLWithPath:d isDirectory:YES]];
			}
			if (defaultName && defaultName[0]) {
				NSString *n = [NSString stringWithUTF8String:defaultName];
				if (n) [panel setNameFieldStringValue:n];
			}
			[panel setLevel:NSModalPanelWindowLevel];
			[panel orderFrontRegardless];
			g_active_panel = panel;
			NSModalResponse r = [panel runModal];
			g_active_panel = nil;
			if (r == NSModalResponseOK && [panel URL] != nil) {
				const char *p = [[[panel URL] path] UTF8String];
				if (p && p[0]) {
					res.paths = (char **)malloc(sizeof(char *));
					if (res.paths) {
						res.paths[0] = strdup(p);
						res.count = 1;
						res.cancelled = 0;
					}
				}
			}
		} else {
			NSOpenPanel *panel = [NSOpenPanel openPanel];
			BOOL isFolder = (kind == 2);
			[panel setCanChooseFiles:!isFolder];
			[panel setCanChooseDirectories:isFolder];
			[panel setCanCreateDirectories:isFolder];
			[panel setAllowsMultipleSelection:(!isFolder && multi != 0)];
			[panel setResolvesAliases:YES];
			if (!isFolder) {
				levi_apply_allowed_extensions(panel, extsCSV, allowOther);
			}
			if (title && title[0]) {
				NSString *t = [NSString stringWithUTF8String:title];
				if (t) {
					[panel setTitle:t];
					[panel setMessage:t];
				}
			}
			if (initialDir && initialDir[0]) {
				NSString *d = [NSString stringWithUTF8String:initialDir];
				if (d) [panel setDirectoryURL:[NSURL fileURLWithPath:d isDirectory:YES]];
			}
			[panel setLevel:NSModalPanelWindowLevel];
			[panel orderFrontRegardless];
			g_active_panel = panel;
			NSModalResponse r = [panel runModal];
			g_active_panel = nil;
			if (r == NSModalResponseOK) {
				NSArray<NSURL *> *urls = [panel URLs];
				NSUInteger n = [urls count];
				if (n > 0) {
					res.paths = (char **)calloc(n, sizeof(char *));
					if (res.paths) {
						int outCount = 0;
						for (NSURL *u in urls) {
							const char *p = [[u path] UTF8String];
							if (p && p[0]) {
								res.paths[outCount++] = strdup(p);
							}
						}
						res.count = outCount;
						res.cancelled = (outCount == 0) ? 1 : 0;
					}
				}
			}
		}
		levi_restore_app_after_panel(prevPID);
	}
	return res;
}
*/
import "C"

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func showNativeDialogPlatform(req DialogRequest) (DialogResult, error) {
	if application.Get() == nil {
		return DialogResult{}, errors.New("application is not ready")
	}

	var kind C.int
	switch req.Kind {
	case "open":
		kind = 0
	case "save":
		kind = 1
	case "folder":
		kind = 2
	default:
		return DialogResult{}, errors.New("unsupported dialog kind")
	}

	filename := strings.TrimSpace(req.DefaultName)
	if req.Kind == "save" && filename != "" && req.DefaultExt != "" && filepath.Ext(filename) == "" {
		filename += "." + req.DefaultExt
	}

	exts, allowOther := NormalizeDialogFilters(req.Filters)
	extsCSV := strings.Join(exts, ";")

	var cTitle, cDir, cName, cExts *C.char
	if req.Title != "" {
		cTitle = C.CString(req.Title)
		defer C.free(unsafe.Pointer(cTitle))
	}
	if req.InitialDir != "" {
		cDir = C.CString(req.InitialDir)
		defer C.free(unsafe.Pointer(cDir))
	}
	if filename != "" {
		cName = C.CString(filename)
		defer C.free(unsafe.Pointer(cName))
	}
	if extsCSV != "" {
		cExts = C.CString(extsCSV)
		defer C.free(unsafe.Pointer(cExts))
	}

	allowFlag := C.int(0)
	if allowOther || len(exts) == 0 {
		allowFlag = 1
	}
	multiFlag := C.int(0)
	if req.Multi {
		multiFlag = 1
	}

	var out DialogResult
	application.InvokeSync(func() {
		cres := C.levi_run_native_panel(kind, cTitle, cDir, cName, cExts, allowFlag, multiFlag)
		defer C.levi_free_dialog_result(&cres)
		if cres.cancelled != 0 || cres.count <= 0 || cres.paths == nil {
			out = DialogResult{Cancelled: true, Paths: []string{}}
			return
		}
		slice := unsafe.Slice(cres.paths, int(cres.count))
		paths := make([]string, 0, int(cres.count))
		for _, cp := range slice {
			if cp != nil {
				paths = append(paths, C.GoString(cp))
			}
		}
		out = DialogResult{Cancelled: len(paths) == 0, Paths: paths}
	})

	if !out.Cancelled && req.Kind == "save" && len(out.Paths) == 1 && req.DefaultExt != "" && filepath.Ext(out.Paths[0]) == "" {
		if _, statErr := os.Stat(out.Paths[0]); os.IsNotExist(statErr) {
			out.Paths[0] += "." + req.DefaultExt
		}
	}
	return out, nil
}

func focusNativeDialogPlatform() {
	if application.Get() == nil {
		return
	}
	C.levi_focus_active_panel()
}
