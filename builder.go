package main

import "fmt"

func BuildTransactionPlan(targetMode string, state SystemState, opts SwitchOptions) (TransactionPlan, error) {

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

			"/etc/X11/xorg.conf.d/90-nvidia.conf",
			"/lib/udev/rules.d/50-remove-nvidia.rules",
			"/lib/udev/rules.d/80-nvidia-pm.rules",
		},
		ToCreate: []FileConfig{},
	}

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

		if state.NvidiaGpuPciBus == "" {
			return plan, fmt.Errorf("Nvidia PCI Bus ID not found. Your GPU is physically powered off (Integrated mode). Please switch to 'hybrid' mode first to wake it up, then reboot and try again")
		}

		if state.IgpuVendor == "intel" {
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: XorgPath, Content: fmt.Sprintf(XorgIntel, state.NvidiaGpuPciBus), Executable: false})
		} else if state.IgpuVendor == "amd" {
			plan.ToCreate = append(plan.ToCreate, FileConfig{Path: XorgPath, Content: fmt.Sprintf(XorgAmd, state.NvidiaGpuPciBus), Executable: false})
		}

		modesetContent := ModesetContent
		if opts.UseNvidiaCurrent {
			modesetContent = ModesetCurrentContent
		}
		plan.ToCreate = append(plan.ToCreate, FileConfig{Path: ModesetPath, Content: modesetContent, Executable: false})

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

	default:
		return plan, fmt.Errorf("unknown target mode: %s", targetMode)
	}

	return plan, nil
}
