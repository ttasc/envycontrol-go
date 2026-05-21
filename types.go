package main

type SwitchOptions struct {
	DisplayManager   string
	ForceComp        bool
	CoolbitsValue    *int
	Rtd3Value        *int
	UseNvidiaCurrent bool
}

type SystemState struct {
	CurrentMode     string `json:"current_mode"`
	IgpuVendor      string `json:"igpu_vendor"`
	NvidiaGpuPciBus string `json:"nvidia_pci_bus"`
}

type FileConfig struct {
	Path       string
	Content    string
	Executable bool
}

type TransactionPlan struct {
	ToRemove []string
	ToCreate []FileConfig
}
