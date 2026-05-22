package main

import "fmt"

// BuildTransactionPlan calculates exactly which files must be removed and created
// to transition the system to the target mode. It performs no disk I/O.
func BuildTransactionPlan(targetMode string, state SystemState, opts SwitchOptions) (TransactionPlan, error) {
	// Base plan always includes a complete cleanup of current configurations
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

// planNvidia populates the plan with rules to force Xorg to route display output through the Nvidia GPU.
func planNvidia(plan *TransactionPlan, state SystemState, opts SwitchOptions) error {
	mod := opts.NvidiaModule

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
	content := fmt.Sprintf(ModesetContent, mod, mod)
	plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: content, Executable: false})

	return nil
}
