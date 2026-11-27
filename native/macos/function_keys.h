/* SPDX-License-Identifier: GPL-3.0-only */
#ifndef LEVI_MACOS_FUNCTION_KEYS_H
#define LEVI_MACOS_FUNCTION_KEYS_H

#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/hidsystem/IOHIDLib.h>
#include <IOKit/hidsystem/IOHIDParameter.h>
#include <IOKit/hidsystem/IOHIDShared.h>
#include <stdbool.h>

static inline io_connect_t levi_open_keyboard_parameters(void) {
    io_service_t service = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching("IOHIDSystem"));
    if (!service) return IO_OBJECT_NULL;
    io_connect_t connection = IO_OBJECT_NULL;
    kern_return_t status = IOServiceOpen(service, mach_task_self(), kIOHIDParamConnectType, &connection);
    IOObjectRelease(service);
    return status == KERN_SUCCESS ? connection : IO_OBJECT_NULL;
}

static inline bool levi_read_function_key_mode(io_connect_t connection, int *mode) {
    CFTypeRef value = NULL;
    bool ok = IOHIDCopyCFTypeParameter(connection, CFSTR(kIOHIDFKeyModeKey), &value) == KERN_SUCCESS &&
        value && CFGetTypeID(value) == CFNumberGetTypeID() &&
        CFNumberGetValue(value, kCFNumberIntType, mode);
    if (value) CFRelease(value);
    return ok;
}

static inline bool levi_set_function_key_mode(io_connect_t connection, int mode) {
    CFNumberRef value = CFNumberCreate(NULL, kCFNumberIntType, &mode);
    if (!value) return false;
    kern_return_t status = IOHIDSetCFTypeParameter(connection, CFSTR(kIOHIDFKeyModeKey), value);
    CFRelease(value);
    return status == KERN_SUCCESS;
}

#endif
