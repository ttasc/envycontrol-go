package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Pre-compiled regular expressions for performance
var (
	dmRegex  = regexp.MustCompile(`ExecStart=(.+)`)
	amdRegex = regexp.MustCompile(`(name:).*(ATI*|AMD*|AMD/ATI)*`)
)

// ProbeNvidiaPciBus executes 'lspci' and parses its output to find the Nvidia
// GPU's PCI Bus ID. It converts the ID from Hexadecimal to Decimal format
// required by Xorg.
func ProbeNvidiaPciBus() (string, error) {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return "", fmt.Errorf("failed to run lspci: %v", err)
	}

	var pciBusID string
	lines := strings.Split(string(out), "\n")

	// Search for the Nvidia VGA/3D controller line
	for _, line := range lines {
		if strings.Contains(line, "NVIDIA") &&
			(strings.Contains(line, "VGA compatible controller") || strings.Contains(line, "3D controller")) {

			parts := strings.Fields(line)
			if len(parts) > 0 {
				// Strip the domain (e.g., "0000:") as Xorg does not use it if it's 0000
				pciBusID = strings.ReplaceAll(parts[0], "0000:", "")
				break
			}
		}
	}

	if pciBusID == "" {
		return "", fmt.Errorf("could not find Nvidia GPU on PCI bus")
	}

	// Safely split and convert Hex values (Bus, Device, Function) to Decimal
	busDevFunc := strings.Split(pciBusID, ":")
	if len(busDevFunc) != 2 {
		return "", fmt.Errorf("invalid PCI format: %s", pciBusID)
	}

	busHex := busDevFunc[0]
	devFunc := strings.Split(busDevFunc[1], ".")
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

// ProbeIgpuVendor scans the PCI bus to determine whether the integrated GPU
// is an Intel or AMD device.
func ProbeIgpuVendor() string {
	out, _ := exec.Command("lspci").Output()
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		if strings.Contains(line, "VGA compatible controller") || strings.Contains(line, "Display controller") {
			if strings.Contains(line, "Intel") {
				return "intel"
			} else if strings.Contains(line, "ATI") || strings.Contains(line, "AMD") || strings.Contains(line, "AMD/ATI") {
				return "amd"
			}
		}
	}
	return ""
}

// ProbeDisplayManager reads the systemd display-manager service configuration
// to automatically detect the running display manager (e.g., sddm, gdm3).
func ProbeDisplayManager() string {
	content, err := os.ReadFile("/etc/systemd/system/display-manager.service")
	if err != nil {
		return ""
	}

	match := dmRegex.FindStringSubmatch(string(content))
	if len(match) > 1 {
		// Split by fields to remove command-line arguments (e.g., "/usr/bin/sddm --debug")
		parts := strings.Fields(match[1])
		if len(parts) > 0 {
			return filepath.Base(parts[0])
		}
	}
	return ""
}

// ProbeAmdIgpuName invokes xrandr to extract the exact provider name for AMD iGPUs.
// This is necessary because AMD provider names vary across driver versions.
func ProbeAmdIgpuName() string {
	if _, err := os.Stat("/usr/bin/xrandr"); os.IsNotExist(err) {
		return ""
	}

	out, _ := exec.Command("xrandr", "--listproviders").Output()
	match := amdRegex.FindString(string(out))

	if match != "" && len(match) > 5 {
		// Strip the "name:" prefix (5 characters)
		return match[5:]
	}
	return ""
}

// GenerateXrandrScript prepares the xrandr setup script string needed
// to route Nvidia GPU output through the integrated GPU.
func GenerateXrandrScript(igpuVendor string) string {
	if igpuVendor == "intel" {
		return fmt.Sprintf(NvidiaXrandrScript, "modesetting")
	} else if igpuVendor == "amd" {
		amdIgpuName := ProbeAmdIgpuName()
		if amdIgpuName != "" {
			return fmt.Sprintf(NvidiaXrandrScript, amdIgpuName)
		}
	}

	// Fallback to modesetting
	return fmt.Sprintf(NvidiaXrandrScript, "modesetting")
}
