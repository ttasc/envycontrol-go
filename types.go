package main

// SwitchOptions contains the user-provided configuration flags and overrides
// passed through the CLI.
type SwitchOptions struct {
	DisplayManager   string // Manually specified display manager (e.g., sddm, gdm)
	ForceComp        bool   // Flag to enable ForceCompositionPipeline in Nvidia mode
	CoolbitsValue    *int   // Optional Coolbits integer value for GPU overclocking/fan control
	Rtd3Value        *int   // Optional Runtime D3 power management level for Hybrid mode
	UseNvidiaCurrent bool   // Flag to use 'nvidia-current' module names (mostly for Debian)
}

// SystemState represents the actual, persistent hardware and software state
// of the machine. It acts as the "Source of Truth".
type SystemState struct {
	CurrentMode     string `json:"current_mode"`   // The currently active graphics mode (integrated, hybrid, nvidia)
	IgpuVendor      string `json:"igpu_vendor"`    // The vendor of the integrated GPU (intel or amd)
	NvidiaGpuPciBus string `json:"nvidia_pci_bus"` // The formatted PCI Bus ID of the Nvidia dGPU (e.g., PCI:1:0:0)
}

// FileConfig represents a single file that needs to be written to the filesystem.
type FileConfig struct {
	Path       string // Absolute path where the file will be written
	Content    string // The raw string content of the file
	Executable bool   // Whether the file needs +x (0755) permissions
}

// TransactionPlan acts as a blueprint for the Transaction Engine.
// It explicitly defines which legacy files to remove and which new files to create,
// completely decoupling the calculation of the config from the actual disk I/O.
type TransactionPlan struct {
	ToRemove []string     // List of absolute file paths to delete
	ToCreate []FileConfig // List of new files to atomically write
}
