package main

import "fmt"

// BuildTransactionPlan calculates exactly which files must be removed and created
// to transition the system to the target mode. It performs no disk I/O.
func BuildTransactionPlan(targetMode string, state SystemState, opts SwitchOptions) (TransactionPlan, error) {
	// Base plan always includes a complete cleanup of previous configurations
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

			// Legacy files from earlier application versions
			"/etc/X11/xorg.conf.d/90-nvidia.conf",
			"/lib/udev/rules.d/50-remove-nvidia.rules",
			"/lib/udev/rules.d/80-nvidia-pm.rules",
		},
		ToCreate: []FileConfig{},
	}

	switch targetMode {
	case "integrated":
		planIntegrated(&plan)
	case "hybrid":
		planHybrid(&plan, opts)
	case "nvidia":
		if err := planNvidia(&plan, state, opts); err != nil {
			return plan, err
		}
	default:
		return plan, fmt.Errorf("unknown target mode: %s", targetMode)
	}

	return plan, nil
}

// planIntegrated populates the plan with rules to entirely power down the Nvidia GPU.
func planIntegrated(plan *TransactionPlan) {
	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: BlacklistPath, Content: BlacklistContent, Executable: false})
	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: UdevIntegratedPath, Content: UdevIntegrated, Executable: false})
}

// planHybrid populates the plan with rules for Prime Render Offload and RTD3 dynamic power management.
func planHybrid(plan *TransactionPlan, opts SwitchOptions) {
	// Basic Hybrid
	if opts.Rtd3Value == nil {
		content := ModesetContent
		if opts.UseNvidiaCurrent {
			content = ModesetCurrentContent
		}
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})
		return
	}

	// Hybrid with RTD3 Power Management
	content := fmt.Sprintf(ModesetRtd3, *opts.Rtd3Value)
	if opts.UseNvidiaCurrent {
		content = fmt.Sprintf(ModesetCurrentRtd3, *opts.Rtd3Value)
	}

	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})
	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: UdevPmPath, Content: UdevPmContent, Executable: false})
}

// planNvidia populates the plan with rules to force Xorg to route all display output through the Nvidia GPU.
func planNvidia(plan *TransactionPlan, state SystemState, opts SwitchOptions) error {
	// Nvidia mode fundamentally requires a known PCI Bus ID to configure Xorg
	if state.NvidiaGpuPciBus == "" {
		return fmt.Errorf("Nvidia PCI Bus ID not found. Your GPU is physically powered off (Integrated mode). Please switch to 'hybrid' mode first to wake it up, then reboot and try again")
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

	// Optional Nvidia features (Composition Pipeline & Overclocking bits)
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

	// Display Manager Xrandr Routing Hacks
	dm := opts.DisplayManager
	if dm == "" {
		dm = ProbeDisplayManager()
	}

	xrandrScript := GenerateXrandrScript(state.IgpuVendor)
	if dm == "sddm" {
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: SddmXsetupPath, Content: xrandrScript, Executable: true})
	} else if dm == "lightdm" {
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: LightdmScriptPath, Content: xrandrScript, Executable: true})
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: LightdmConfigPath, Content: LightdmConfigContent, Executable: false})
	}

	return nil
}
