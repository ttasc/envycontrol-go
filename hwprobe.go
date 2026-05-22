package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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
