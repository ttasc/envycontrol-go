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

// Lấy PCI Bus ID của card Nvidia bằng cách bóc tách output lệnh lspci
func GetNvidiaGpuPciBus() string {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		LogError("Failed to run 'lspci': %v", err)
	}

	lines := strings.Split(string(out), "\n")
	var pciBusID string
	found := false

	for _, line := range lines {
		// Tìm dòng có chữ NVIDIA và là card đồ họa
		if strings.Contains(line, "NVIDIA") && (strings.Contains(line, "VGA compatible controller") || strings.Contains(line, "3D controller")) {
			// Xóa các số 0 vô nghĩa ở đầu (vd: "0000:01:00.0" -> "01:00.0")
			parts := strings.Fields(line)
			if len(parts) > 0 {
				pciBusID = strings.ReplaceAll(parts[0], "0000:", "")
				LogInfo("Found Nvidia GPU at %s", pciBusID)
				found = true
				break
			}
		}
	}

	if !found {
		LogError("Could not find Nvidia GPU")
		fmt.Println("Try switching to hybrid mode first!")
		os.Exit(1)
	}

	// Cần chuyển BusID từ lspci (01:00.0) sang chuẩn của Xorg (PCI:1:0:0)
	// Xorg yêu cầu chuyển từ số Hex (Cơ số 16) sang Decimal (Cơ số 10)
	busDevFunc := strings.Split(pciBusID, ":")
	busHex := busDevFunc[0]
	devFunc := strings.Split(busDevFunc[1], ".")
	devHex := devFunc[0]
	funcHex := devFunc[1]

	busDec, err1 := strconv.ParseInt(busHex, 16, 64)
	devDec, err2 := strconv.ParseInt(devHex, 16, 64)
	funcDec, err3 := strconv.ParseInt(funcHex, 16, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		LogError("Failed to parse PCI Bus ID format. Raw hex values: %s:%s.%s", busHex, devHex, funcHex)
		os.Exit(1)
	}

	return fmt.Sprintf("PCI:%d:%d:%d", busDec, devDec, funcDec)
}

// Tìm Vendor của iGPU (Card đồ họa tích hợp)
func GetIgpuVendor() string {
	out, _ := exec.Command("lspci").Output()
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		if strings.Contains(line, "VGA compatible controller") || strings.Contains(line, "Display controller") {
			if strings.Contains(line, "Intel") {
				LogInfo("Found Intel iGPU")
				return "intel"
			} else if strings.Contains(line, "ATI") || strings.Contains(line, "AMD") || strings.Contains(line, "AMD/ATI") {
				LogInfo("Found AMD iGPU")
				return "amd"
			}
		}
	}

	LogWarning("Could not find Intel or AMD iGPU")
	return ""
}

// Đọc service của systemd để tìm ra Display Manager hệ thống đang chạy
func GetDisplayManager() string {
	content, err := os.ReadFile("/etc/systemd/system/display-manager.service")
	if err != nil {
		LogWarning("Display Manager detection is not available")
		return ""
	}

	// Regex tìm đúng dòng ExecStart=
	re := regexp.MustCompile(`ExecStart=(.+)`)
	match := re.FindStringSubmatch(string(content))

	if len(match) > 1 {
		// Chỉ lấy tên file thực thi cuối cùng (vd: /usr/sbin/gdm3 -> gdm3)
		dm := filepath.Base(strings.TrimSpace(match[1]))
		LogInfo("Found %s Display Manager", dm)
		return dm
	}

	return ""
}

// Tìm tên iGPU AMD trong xrandr để map đúng output cho màn hình
func GetAmdIgpuName() string {
	if _, err := os.Stat("/usr/bin/xrandr"); os.IsNotExist(err) {
		LogWarning("The 'xrandr' command is not available. Make sure the package is installed!")
		return ""
	}

	out, err := exec.Command("xrandr", "--listproviders").Output()
	if err != nil {
		LogWarning("Failed to run the 'xrandr' command.")
	}

	// Regex y hệt code gốc r'(name:).*(ATI*|AMD*|AMD\/ATI)*'
	re := regexp.MustCompile(`(name:).*(ATI*|AMD*|AMD/ATI)*`)
	match := re.FindString(string(out))

	if match != "" {
		// Bỏ 5 ký tự đầu tiên ("name:") để lấy tên card
		return match[5:]
	}

	LogWarning("Could not find AMD iGPU in 'xrandr' output.")
	return ""
}

// Sinh nội dung script xrandr dựa trên loại iGPU
func GenerateXrandrScript(igpuVendor string) string {
	if igpuVendor == "intel" {
		return fmt.Sprintf(NvidiaXrandrScript, "modesetting")
	} else if igpuVendor == "amd" {
		amdIgpuName := GetAmdIgpuName()
		if amdIgpuName != "" {
			return fmt.Sprintf(NvidiaXrandrScript, amdIgpuName)
		}
		// Dự phòng nếu không tìm thấy tên AMD
		return fmt.Sprintf(NvidiaXrandrScript, "modesetting")
	}

	// Mặc định
	return fmt.Sprintf(NvidiaXrandrScript, "modesetting")
}
