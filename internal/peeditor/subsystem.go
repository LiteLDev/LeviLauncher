package peeditor

import (
	"context"
	"encoding/binary"
	"os"
)

const (
	IMAGE_SUBSYSTEM_UNKNOWN                  = 0
	IMAGE_SUBSYSTEM_NATIVE                   = 1
	IMAGE_SUBSYSTEM_WINDOWS_GUI              = 2
	IMAGE_SUBSYSTEM_WINDOWS_CUI              = 3
	IMAGE_SUBSYSTEM_OS2_CUI                  = 5
	IMAGE_SUBSYSTEM_POSIX_CUI                = 7
	IMAGE_SUBSYSTEM_NATIVE_WINDOWS           = 8
	IMAGE_SUBSYSTEM_WINDOWS_CE_GUI           = 9
	IMAGE_SUBSYSTEM_EFI_APPLICATION          = 10
	IMAGE_SUBSYSTEM_EFI_BOOT_SERVICE_DRIVER  = 11
	IMAGE_SUBSYSTEM_EFI_RUNTIME_DRIVER       = 12
	IMAGE_SUBSYSTEM_EFI_ROM                  = 13
	IMAGE_SUBSYSTEM_XBOX                     = 14
	IMAGE_SUBSYSTEM_WINDOWS_BOOT_APPLICATION = 16
)

func SetSubsystem(ctx context.Context, exePath string, console bool) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		return false
	}

	if len(data) < 64 {
		return false
	}

	if data[0] != 'M' || data[1] != 'Z' {
		return false
	}

	peOffset := int(binary.LittleEndian.Uint32(data[60:64]))
	if peOffset+4 >= len(data) {
		return false
	}

	if data[peOffset] != 'P' || data[peOffset+1] != 'E' || data[peOffset+2] != 0 || data[peOffset+3] != 0 {
		return false
	}

	coffHeaderStart := peOffset + 4

	optionalHeaderStart := coffHeaderStart + 20

	if optionalHeaderStart+2 >= len(data) {
		return false
	}

	magic := binary.LittleEndian.Uint16(data[optionalHeaderStart : optionalHeaderStart+2])

	var subsystemOffset int
	if magic == 0x10b {
		subsystemOffset = optionalHeaderStart + 68
	} else if magic == 0x20b {
		subsystemOffset = optionalHeaderStart + 68
	} else {
		return false
	}

	if subsystemOffset+2 >= len(data) {
		return false
	}

	var targetSubsystem uint16
	if console {
		targetSubsystem = IMAGE_SUBSYSTEM_WINDOWS_CUI
	} else {
		targetSubsystem = IMAGE_SUBSYSTEM_WINDOWS_GUI
	}

	currentSubsystem := binary.LittleEndian.Uint16(data[subsystemOffset : subsystemOffset+2])

	if currentSubsystem == targetSubsystem {
		return true
	}

	binary.LittleEndian.PutUint16(data[subsystemOffset:subsystemOffset+2], targetSubsystem)

	if err := os.WriteFile(exePath, data, 0644); err != nil {
		return false
	}
	return true
}
