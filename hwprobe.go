package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	vendorNvidia = "0x10de"
	vendorIntel  = "0x8086"
	vendorAMD    = "0x1002"
)

var SysfsPciDevicesDir = "/sys/bus/pci/devices"

// readSysfsFile is a helper function to safely read a single-line text file
// from the Linux SysFS, trimming any trailing whitespace or newlines.
func readSysfsFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// isGraphicsClass checks if the PCI device class indicates a VGA or 3D controller.
// SysFS exposes the class as a 6-digit hex string starting with 0x (e.g., "0x030000").
func isGraphicsClass(class string) bool {
	return strings.HasPrefix(class, "0x0300") || strings.HasPrefix(class, "0x0302")
}

// ProbeNvidiaPciBus scans the Linux SysFS to find the Nvidia GPU's PCI Bus ID.
// It converts the ID from the directory name (Hex) to the Decimal format required by Xorg.
// Returns an error if the GPU is physically powered off (Integrated mode) or missing.
func ProbeNvidiaPciBus() (string, error) {
	entries, err := os.ReadDir(SysfsPciDevicesDir)
	if err != nil {
		return "", fmt.Errorf("failed to read sysfs PCI directory: %v", err)
	}

	var pciBusID string

	for _, entry := range entries {
		devicePath := filepath.Join(SysfsPciDevicesDir, entry.Name())
		vendor := readSysfsFile(filepath.Join(devicePath, "vendor"))
		class := readSysfsFile(filepath.Join(devicePath, "class"))

		if vendor == vendorNvidia && isGraphicsClass(class) {
			pciBusID = entry.Name() // e.g., "0000:01:00.0"
			break
		}
	}

	if pciBusID == "" {
		return "", fmt.Errorf("nvidia GPU not found on PCI bus (might be powered off)")
	}

	// Safely split and convert Hex values (Domain:Bus:Device.Function) to Decimal
	parts := strings.Split(pciBusID, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid PCI format: %s", pciBusID)
	}

	busHex := parts[1]
	devFunc := strings.Split(parts[2], ".")
	if len(devFunc) != 2 {
		return "", fmt.Errorf("invalid PCI dev.func format: %s", pciBusID)
	}

	busDec, err1 := strconv.ParseInt(busHex, 16, 64)
	devDec, err2 := strconv.ParseInt(devFunc[0], 16, 64)
	funcDec, err3 := strconv.ParseInt(devFunc[1], 16, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return "", fmt.Errorf("failed to parse PCI Bus ID hex values: %s", pciBusID)
	}

	return fmt.Sprintf("PCI:%d:%d:%d", busDec, devDec, funcDec), nil
}

// ProbeIgpuVendor scans the Linux SysFS to determine whether the integrated GPU
// is an Intel or AMD device.
func ProbeIgpuVendor() string {
	entries, err := os.ReadDir(SysfsPciDevicesDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		devicePath := filepath.Join(SysfsPciDevicesDir, entry.Name())
		class := readSysfsFile(filepath.Join(devicePath, "class"))

		if isGraphicsClass(class) {
			vendor := readSysfsFile(filepath.Join(devicePath, "vendor"))
			switch vendor {
			case vendorIntel:
				return "intel"
			case vendorAMD:
				return "amd"
			}
		}
	}
	return ""
}

// GuessCurrentMode uses real-time file presence heuristics to determine the active mode.
func GuessCurrentMode() string {
	fileExists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	blacklist := fileExists(BlacklistPath)
	udevInteg := fileExists(UdevIntegratedPath)
	xorg := fileExists(XorgPath)
	modeset := fileExists(ModesetPath)

	if blacklist && udevInteg {
		return "integrated"
	} else if xorg && modeset {
		return "nvidia"
	}
	return "hybrid"
}
