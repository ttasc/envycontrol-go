package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CacheData biểu diễn cấu trúc JSON của file cache
type CacheData struct {
	NvidiaGpuPciBus string `json:"nvidia_gpu_pci_bus"`
}

// Kiểm tra xem một file có tồn tại hay không
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Xác định mode đồ họa hiện tại bằng heuristic (kiểm tra sự tồn tại của file config)
func GetCurrentMode() string {
	mode := "hybrid" // Default

	blacklistExists := fileExists(BlacklistPath)
	udevIntegratedExists := fileExists(UdevIntegratedPath)
	oldUdevExists := fileExists("/lib/udev/rules.d/50-remove-nvidia.rules")
	xorgExists := fileExists(XorgPath)
	modesetExists := fileExists(ModesetPath)

	if blacklistExists && (udevIntegratedExists || oldUdevExists) {
		mode = "integrated"
	} else if xorgExists && modesetExists {
		mode = "nvidia"
	}

	return mode
}

// Ghi chuỗi JSON xuống file cache
func writeCacheFile(pciBus string) {
	dir := filepath.Dir(CacheFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		LogError("Failed to create cache directory: %v", err)
		return
	}

	data := CacheData{
		NvidiaGpuPciBus: pciBus,
	}

	// Marshal với Indent để JSON đẹp như Python làm (indent=4)
	file, _ := json.MarshalIndent(data, "", "    ")

	err := os.WriteFile(CacheFilePath, file, 0644)
	if err != nil {
		LogError("Failed to write cache file: %v", err)
		return
	}

	LogDebug("Created file %s", CacheFilePath)
}

// --- Các hàm CLI Action về Cache ---

// Tương đương với --cache-create
func CreateCache() {
	if GetCurrentMode() != "hybrid" {
		LogError("--cache-create requires that the system be in the hybrid Optimus mode")
		os.Exit(1) // Tái tạo lại ValueError của Python
	}

	pciBus := GetNvidiaGpuPciBus()
	writeCacheFile(pciBus)
}

// Tương đương với --cache-delete
func DeleteCache() {
	os.Remove(CacheFilePath)
	// Xóa luôn thư mục cha nếu trống (giống os.removedirs của Python)
	os.Remove(filepath.Dir(CacheFilePath))
	LogDebug("Removed file %s", CacheFilePath)
}

// Tương đương với --cache-query
func ShowCache() {
	if fileExists(CacheFilePath) {
		content, _ := os.ReadFile(CacheFilePath)
		fmt.Print(string(content))
	} else {
		fmt.Printf("ERROR: Could not read %s\n", CacheFilePath)
	}
}

// Hàm này thay thế ContextManager "adapter()" của bản Python
// Nó sẽ được gọi trước khi bắt đầu thực hiện Switch mode
func SetupCacheAdapter() {
	// Nếu hệ thống đang ở Hybrid mode -> Luôn tạo lại Cache mới nhất (đề phòng card bị cắm nhầm slot PCI)
	if GetCurrentMode() == "hybrid" {
		pciBus := GetNvidiaGpuPciBus()
		writeCacheFile(pciBus)
	}
}

// Hàm này lấy PCI ID an toàn: Đọc từ cache trước, nếu không có mới tìm bằng lspci
// (Thay thế cho hành động monkey patch global get_nvidia_gpu_pci_bus của tác giả gốc)
func ReadPciBusWithCache() string {
	if fileExists(CacheFilePath) {
		content, err := os.ReadFile(CacheFilePath)
		if err != nil {
			LogError("Failed to read cache file: %v", err)
			os.Exit(1)
		}

		var data CacheData
		if err := json.Unmarshal(content, &data); err != nil {
			LogError("Failed to parse cache file: %v", err)
			os.Exit(1)
		}

		// Trong quá trình read có thể log debug
		// LogDebug("Using cached PCI ID: %s", data.NvidiaGpuPciBus)
		return data.NvidiaGpuPciBus
	} else if GetCurrentMode() == "hybrid" {
		return GetNvidiaGpuPciBus()
	}

	LogError("No cache present. Operation requires that the system be in the hybrid Optimus mode")
	os.Exit(1)
	return ""
}
