package peeditor

import (
	"context"
	"encoding/binary"
	"fmt"
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
		fmt.Printf("Failed to read file: %v\n", err)
		return false
	}

	if len(data) < 64 {
		fmt.Printf("File too small\n")
		return false
	}

	if data[0] != 'M' || data[1] != 'Z' {
		fmt.Printf("Invalid DOS signature\n")
		return false
	}

	peOffset := int(binary.LittleEndian.Uint32(data[60:64]))
	if peOffset+4 >= len(data) {
		fmt.Printf("Invalid PE header offset: %d\n", peOffset)
		return false
	}

	if data[peOffset] != 'P' || data[peOffset+1] != 'E' || data[peOffset+2] != 0 || data[peOffset+3] != 0 {
		fmt.Printf("Invalid PE signature\n")
		return false
	}

	coffHeaderStart := peOffset + 4

	optionalHeaderStart := coffHeaderStart + 20

	if optionalHeaderStart+2 >= len(data) {
		fmt.Printf("Cannot read magic number\n")
		return false
	}

	magic := binary.LittleEndian.Uint16(data[optionalHeaderStart : optionalHeaderStart+2])
	fmt.Printf("PE Magic: 0x%x\n", magic)

	var subsystemOffset int
	if magic == 0x10b { // PE32
		subsystemOffset = optionalHeaderStart + 68
	} else if magic == 0x20b { // PE32+
		subsystemOffset = optionalHeaderStart + 68
	} else {
		fmt.Printf("Unknown PE format: 0x%x\n", magic)
		return false
	}

	if subsystemOffset+2 >= len(data) {
		fmt.Printf("Subsystem offset out of range: %d\n", subsystemOffset)
		return false
	}

	var targetSubsystem uint16
	if console {
		targetSubsystem = IMAGE_SUBSYSTEM_WINDOWS_CUI
	} else {
		targetSubsystem = IMAGE_SUBSYSTEM_WINDOWS_GUI
	}

	currentSubsystem := binary.LittleEndian.Uint16(data[subsystemOffset : subsystemOffset+2])
	fmt.Printf("Current subsystem: %d, Target subsystem: %d\n", currentSubsystem, targetSubsystem)

	if currentSubsystem == targetSubsystem {
		fmt.Printf("Subsystem already set to target value\n")
		return true
	}

	binary.LittleEndian.PutUint16(data[subsystemOffset:subsystemOffset+2], targetSubsystem)

	if err := os.WriteFile(exePath, data, 0644); err != nil {
		fmt.Printf("Failed to write file: %v\n", err)
		return false
	}

	fmt.Printf("Successfully changed subsystem from %d to %d\n", currentSubsystem, targetSubsystem)
	return true
}
