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

func ProbeNvidiaPciBus() (string, error) {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return "", fmt.Errorf("failed to run lspci: %v", err)
	}

	lines := strings.Split(string(out), "\n")
	var pciBusID string

	for _, line := range lines {
		if strings.Contains(line, "NVIDIA") && (strings.Contains(line, "VGA compatible controller") || strings.Contains(line, "3D controller")) {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				pciBusID = strings.ReplaceAll(parts[0], "0000:", "")
				break
			}
		}
	}

	if pciBusID == "" {
		return "", fmt.Errorf("could not find Nvidia GPU on PCI bus")
	}

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

func ProbeDisplayManager() string {
	content, err := os.ReadFile("/etc/systemd/system/display-manager.service")
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`ExecStart=(.+)`)
	match := re.FindStringSubmatch(string(content))

	if len(match) > 1 {
		parts := strings.Fields(match[1])
		if len(parts) > 0 {
			return filepath.Base(parts[0])
		}
	}
	return ""
}

func ProbeAmdIgpuName() string {
	if _, err := os.Stat("/usr/bin/xrandr"); os.IsNotExist(err) {
		return ""
	}

	out, _ := exec.Command("xrandr", "--listproviders").Output()
	re := regexp.MustCompile(`(name:).*(ATI*|AMD*|AMD/ATI)*`)
	match := re.FindString(string(out))

	if match != "" && len(match) > 5 {
		return match[5:]
	}
	return ""
}

func GenerateXrandrScript(igpuVendor string) string {
	if igpuVendor == "intel" {
		return fmt.Sprintf(NvidiaXrandrScript, "modesetting")
	} else if igpuVendor == "amd" {
		amdIgpuName := ProbeAmdIgpuName()
		if amdIgpuName != "" {
			return fmt.Sprintf(NvidiaXrandrScript, amdIgpuName)
		}
	}
	return fmt.Sprintf(NvidiaXrandrScript, "modesetting")
}
