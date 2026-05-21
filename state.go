package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadState đọc thông tin từ file State. Nếu file bị mất, tự động xây dựng lại từ hệ thống.
func LoadState() SystemState {
	if _, err := os.Stat(StateFilePath); err == nil {
		content, err := os.ReadFile(StateFilePath)
		if err == nil {
			var state SystemState
			if err := json.Unmarshal(content, &state); err == nil {
				return state // Trả về nếu đọc thành công
			}
		}
	}

	// Fallback: Đoán trạng thái và lấy thông tin phần cứng nếu chưa có State File
	return RebuildState()
}

// RebuildState quét hệ thống thực tế để tái tạo SystemState
func RebuildState() SystemState {
	state := SystemState{
		CurrentMode: guessCurrentMode(),
		IgpuVendor:  ProbeIgpuVendor(),
	}

	// Chỉ lấy được PCI ID thực tế khi card đồ họa Nvidia đang được bật
	if state.CurrentMode != "integrated" {
		if pci, err := ProbeNvidiaPciBus(); err == nil {
			state.NvidiaGpuPciBus = pci
		}
	}
	return state
}

// SaveState lưu trữ SystemState xuống đĩa an toàn
func SaveState(state SystemState) error {
	dir := filepath.Dir(StateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Format JSON đẹp, thụt đầu dòng (Indent)
	data, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(StateFilePath, data, 0644)
}

// guessCurrentMode (Heuristic) kiểm tra file trên đĩa cứng như cách cũ
func guessCurrentMode() string {
	fileExists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	blacklist := fileExists(BlacklistPath)
	udevInteg := fileExists(UdevIntegratedPath) || fileExists("/lib/udev/rules.d/50-remove-nvidia.rules")
	xorg := fileExists(XorgPath)
	modeset := fileExists(ModesetPath)

	if blacklist && udevInteg {
		return "integrated"
	} else if xorg && modeset {
		return "nvidia"
	}
	return "hybrid"
}
