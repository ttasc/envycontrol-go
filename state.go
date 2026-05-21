package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func LoadState() SystemState {
	if _, err := os.Stat(StateFilePath); err == nil {
		content, err := os.ReadFile(StateFilePath)
		if err == nil {
			var state SystemState
			if err := json.Unmarshal(content, &state); err == nil {
				return state
			}
		}
	}

	return RebuildState()
}

func RebuildState() SystemState {
	state := SystemState{
		CurrentMode: GuessCurrentMode(),
		IgpuVendor:  ProbeIgpuVendor(),
	}

	if state.CurrentMode != "integrated" {
		if pci, err := ProbeNvidiaPciBus(); err == nil {
			state.NvidiaGpuPciBus = pci
		}
	}
	return state
}

func SaveState(newState SystemState) error {
	dir := filepath.Dir(StateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

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
