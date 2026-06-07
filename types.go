package main

// SwitchOptions contains the configuration flags and environment overrides
// required for transitioning graphics modes.
type SwitchOptions struct {
	NvidiaModule string // Target kernel module name (e.g., "nvidia", "nvidia-current")
	Rtd3Value    *int   // Optional Runtime D3 power management level (0, 1, 2, 3)
	IsWayland    bool   // Flag to optimize hardware configurations for Wayland instead of X11
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
