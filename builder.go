package main

import "fmt"

// BuildTransactionPlan tính toán chính xác hệ thống cần xóa file nào và tạo file nào.
func BuildTransactionPlan(targetMode string, state SystemState, opts SwitchOptions) (TransactionPlan, error) {
	// 1. LUÔN LUÔN lên kế hoạch dọn dẹp sạch sẽ toàn bộ config cũ (Cleanup Step)
	plan := TransactionPlan{
		ToRemove: []string{
			BlacklistPath,
			UdevIntegratedPath,
			UdevPmPath,
			XorgPath,
			ExtraXorgPath,
			ModesetPath,
			LightdmScriptPath,
			LightdmConfigPath,
			// Các file rác thừa kế từ bản cũ
			"/etc/X11/xorg.conf.d/90-nvidia.conf",
			"/lib/udev/rules.d/50-remove-nvidia.rules",
			"/lib/udev/rules.d/80-nvidia-pm.rules",
		},
		ToCreate: []FileConfig{},
	}

	// 2. Tính toán các file cần sinh ra dựa trên targetMode
	switch targetMode {
	case "integrated":
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: BlacklistPath, Content: BlacklistContent, Executable: false})
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: UdevIntegratedPath, Content: UdevIntegrated, Executable: false})

	case "hybrid":
		if opts.Rtd3Value == nil {
			content := ModesetContent
			if opts.UseNvidiaCurrent {
				content = ModesetCurrentContent
			}
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})
		} else {
			content := fmt.Sprintf(ModesetRtd3, *opts.Rtd3Value)
			if opts.UseNvidiaCurrent {
				content = fmt.Sprintf(ModesetCurrentRtd3, *opts.Rtd3Value)
			}
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: UdevPmPath, Content: UdevPmContent, Executable: false})
		}

	case "nvidia":
		// Điều kiện tiên quyết: Phải có PCI ID để cấu hình Xorg Server
		if state.NvidiaGpuPciBus == "" {
			return plan, fmt.Errorf("missing Nvidia PCI Bus ID in state. Please switch to hybrid mode first")
		}

		// Xorg Base Config
		if state.IgpuVendor == "intel" {
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: XorgPath, Content: fmt.Sprintf(XorgIntel, state.NvidiaGpuPciBus), Executable: false})
		} else if state.IgpuVendor == "amd" {
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: XorgPath, Content: fmt.Sprintf(XorgAmd, state.NvidiaGpuPciBus), Executable: false})
		}

		// Modeset Config
		modesetContent := ModesetContent
		if opts.UseNvidiaCurrent {
			modesetContent = ModesetCurrentContent
		}
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: modesetContent, Executable: false})

		// Extra Xorg Config (ForceComp & Coolbits)
		if opts.ForceComp || opts.CoolbitsValue != nil {
			extraConfig := ExtraXorgContent
			if opts.ForceComp {
				extraConfig += ForceComp
			}
			if opts.CoolbitsValue != nil {
				extraConfig += fmt.Sprintf(Coolbits, *opts.CoolbitsValue)
			}
			extraConfig += "EndSection\n"
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ExtraXorgPath, Content: extraConfig, Executable: false})
		}

		// Display Manager Scripts (Hack xuất hình)
		dm := opts.DisplayManager
		if dm == "" {
			dm = ProbeDisplayManager()
		}

		xrandrScript := GenerateXrandrScript(state.IgpuVendor)
		if dm == "sddm" {
			// File Xsetup cần quyền +x (executable)
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: SddmXsetupPath, Content: xrandrScript, Executable: true})
		} else if dm == "lightdm" {
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: LightdmScriptPath, Content: xrandrScript, Executable: true})
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: LightdmConfigPath, Content: LightdmConfigContent, Executable: false})
		}

	default:
		return plan, fmt.Errorf("unknown target mode: %s", targetMode)
	}

	return plan, nil
}
