// hwprobe_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper to create fake sysfs device entries
func createFakePCIDevice(t *testing.T, baseDir, devID, vendorID, classID string) {
	devDir := filepath.Join(baseDir, devID)
	if err := os.MkdirAll(devDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(devDir, "vendor"), []byte(vendorID+"\n"), 0644)
	os.WriteFile(filepath.Join(devDir, "class"), []byte(classID+"\n"), 0644)
}

func TestProbeNvidiaPciBus(t *testing.T) {
	tmpDir := t.TempDir()

	// Override the package variable (Assuming refactor applied)
	SysfsPciDevicesDir = tmpDir

	// Scenario 1: Nvidia GPU present (VGA controller class)
	createFakePCIDevice(t, tmpDir, "0000:01:00.0", "0x10de", "0x030000")
	// Noise/Other devices
	createFakePCIDevice(t, tmpDir, "0000:00:1f.3", "0x8086", "0x040300") // Intel Audio

	pci, err := ProbeNvidiaPciBus()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if pci != "PCI:1:0:0" {
		t.Errorf("Expected 'PCI:1:0:0', got '%s'", pci)
	}
}

func TestProbeNvidiaPciBus_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	SysfsPciDevicesDir = tmpDir

	// Only Intel iGPU present
	createFakePCIDevice(t, tmpDir, "0000:00:02.0", "0x8086", "0x030000")

	_, err := ProbeNvidiaPciBus()
	if err == nil {
		t.Fatal("Expected error when Nvidia GPU is missing/powered off")
	}
}

func TestProbeIgpuVendor(t *testing.T) {
	tmpDir := t.TempDir()
	SysfsPciDevicesDir = tmpDir

	createFakePCIDevice(t, tmpDir, "0000:00:02.0", "0x8086", "0x030000") // Intel
	createFakePCIDevice(t, tmpDir, "0000:01:00.0", "0x10de", "0x030000") // Nvidia

	vendor := ProbeIgpuVendor()
	if vendor != "intel" {
		t.Errorf("Expected 'intel', got '%s'", vendor)
	}

	// Reset and test AMD
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	createFakePCIDevice(t, tmpDir, "0000:04:00.0", "0x1002", "0x030000") // AMD

	vendor = ProbeIgpuVendor()
	if vendor != "amd" {
		t.Errorf("Expected 'amd', got '%s'", vendor)
	}
}

func TestGuessCurrentMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Override paths dynamically
	BlacklistPath = filepath.Join(tmpDir, "blacklist.conf")
	UdevIntegratedPath = filepath.Join(tmpDir, "udev.rules")
	XorgPath = filepath.Join(tmpDir, "xorg.conf")
	ModesetPath = filepath.Join(tmpDir, "modeset.conf")

	// Hybrid (Default - no files)
	if mode := GuessCurrentMode(); mode != "hybrid" {
		t.Errorf("Expected hybrid, got %s", mode)
	}

	// Simulate Integrated
	os.WriteFile(BlacklistPath, []byte(""), 0644)
	os.WriteFile(UdevIntegratedPath, []byte(""), 0644)
	if mode := GuessCurrentMode(); mode != "integrated" {
		t.Errorf("Expected integrated, got %s", mode)
	}

	// Reset
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)

	// Simulate Nvidia
	os.WriteFile(XorgPath, []byte(""), 0644)
	os.WriteFile(ModesetPath, []byte(""), 0644)
	if mode := GuessCurrentMode(); mode != "nvidia" {
		t.Errorf("Expected nvidia, got %s", mode)
	}
}
