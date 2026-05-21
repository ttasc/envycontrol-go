package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadState retrieves the SystemState from the disk.
// If the state file is missing or corrupted, it automatically rebuilds
// the state using system heuristics.
func LoadState() SystemState {
	content, err := os.ReadFile(StateFilePath)
	if err == nil {
		var state SystemState
		if err := json.Unmarshal(content, &state); err == nil {
			return state
		}
	}

	// Fallback if file is missing or invalid
	return RebuildState()
}

// RebuildState probes the active system environment to reconstruct the SystemState.
func RebuildState() SystemState {
	state := SystemState{
		CurrentMode: GuessCurrentMode(),
		IgpuVendor:  ProbeIgpuVendor(),
	}

	// We can only reliably probe the Nvidia PCI Bus if the card is powered on
	if state.CurrentMode != "integrated" {
		if pci, err := ProbeNvidiaPciBus(); err == nil {
			state.NvidiaGpuPciBus = pci
		}
	}
	return state
}

// SaveState persists the SystemState to disk.
// It ensures that an existing Nvidia PCI Bus ID is not accidentally erased.
func SaveState(newState SystemState) error {
	dir := filepath.Dir(StateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Protect against losing the PCI Bus ID if overwriting from Integrated mode
	if content, err := os.ReadFile(StateFilePath); err == nil {
		var oldState SystemState
		if json.Unmarshal(content, &oldState) == nil {
			if newState.NvidiaGpuPciBus == "" && oldState.NvidiaGpuPciBus != "" {
				newState.NvidiaGpuPciBus = oldState.NvidiaGpuPciBus
			}
		}
	}

	data, err := json.MarshalIndent(newState, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(StateFilePath, data, 0644)
}

// GuessCurrentMode uses file presence heuristics to guess the active graphics mode.
// It acts as a fallback when the state file is unavailable or when the user
// runs the application in read-only query mode.
func GuessCurrentMode() string {
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
