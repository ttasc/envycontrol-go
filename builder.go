package main

import "fmt"

// BuildTransactionPlan calculates exactly which files must be removed and created
// to transition the system to the target mode. It performs no disk I/O.
func BuildTransactionPlan(targetMode string, pciBus string, igpuVendor string, opts SwitchOptions) (TransactionPlan, error) {
	plan := TransactionPlan{
		ToRemove: []string{
			BlacklistPath,
			UdevIntegratedPath,
			UdevPmPath,
			XorgPath,
			ModesetPath,
		},
		ToCreate: []FileConfig{},
	}

	switch targetMode {
	case "integrated":
		planIntegrated(&plan)
	case "hybrid":
		planHybrid(&plan, opts)
	case "nvidia":
		if err := planNvidia(&plan, pciBus, igpuVendor, opts); err != nil {
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
	mod := opts.NvidiaModule

	if opts.Rtd3Value == nil {
		// Basic Hybrid without explicit RTD3 override
		content := fmt.Sprintf(ModesetContent, mod, mod)
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})
		return
	}

	// Hybrid with explicit RTD3 Power Management
	content := fmt.Sprintf(ModesetRtd3, mod, mod, *opts.Rtd3Value, mod)
	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})
	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: UdevPmPath, Content: UdevPmContent, Executable: false})
}

// planNvidia populates the plan with rules to force routing display output through the Nvidia GPU.
// If the IsWayland option is enabled, it skips generating the legacy Xorg configuration to allow
// the Wayland compositor to seamlessly manage DRM routing natively.
func planNvidia(plan *TransactionPlan, pciBus string, igpuVendor string, opts SwitchOptions) error {
	mod := opts.NvidiaModule

	if pciBus == "" {
		return fmt.Errorf("nvidia PCI Bus ID is missing")
	}

	// Only generate Xorg configurations if we are NOT running in Wayland optimization mode
	if !opts.IsWayland {
		switch igpuVendor {
		case "intel":
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: XorgPath, Content: fmt.Sprintf(XorgIntel, pciBus), Executable: false})
		case "amd":
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: XorgPath, Content: fmt.Sprintf(XorgAmd, pciBus), Executable: false})
		}
	}

	// Always generate kernel modprobe parameters (required for both X11 and Wayland)
	content := fmt.Sprintf(ModesetContent, mod, mod)
	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})

	return nil
}
