package main

// SwitchOptions chứa các tùy chọn do người dùng truyền vào qua CLI
type SwitchOptions struct {
	DisplayManager   string
	ForceComp        bool
	CoolbitsValue    *int
	Rtd3Value        *int
	UseNvidiaCurrent bool
}

// SystemState lưu trữ sự thật (Source of Truth) về phần cứng của máy
type SystemState struct {
	CurrentMode     string `json:"current_mode"`
	IgpuVendor      string `json:"igpu_vendor"`
	NvidiaGpuPciBus string `json:"nvidia_pci_bus"`
}

// FileConfig biểu diễn một file sẽ được ghi xuống đĩa
type FileConfig struct {
	Path       string
	Content    string
	Executable bool
}

// TransactionPlan là một "Bản kế hoạch" chứa danh sách các file cần xóa và tạo.
// Bản kế hoạch này sẽ được sinh ra ở tầng Logic, sau đó đưa cho tầng I/O thực thi.
type TransactionPlan struct {
	ToRemove []string
	ToCreate []FileConfig
}
