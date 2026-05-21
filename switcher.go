package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func SwitchMode(targetMode string, opts SwitchOptions) {
	fmt.Printf("Switching to %s mode\n", targetMode)

	state := LoadState()

	if targetMode == "integrated" || targetMode == "hybrid" {
		RestoreSddmXsetup()
	} else if targetMode == "nvidia" {
		dm := opts.DisplayManager
		if dm == "" {
			dm = ProbeDisplayManager()
		}
		if dm == "sddm" {
			BackupSddmXsetup()
		}
	}

	ctxBg := context.Background()
	if targetMode == "integrated" {
		exitCode, _ := RunCommand(ctxBg, !Verbose, "systemctl", "disable", "nvidia-persistenced.service")
		if exitCode == 0 {
			fmt.Println("Successfully disabled nvidia-persistenced.service")
		} else {
			LogError("An error occurred while disabling nvidia-persistenced.service")
		}
	} else {
		exitCode, _ := RunCommand(ctxBg, !Verbose, "systemctl", "enable", "nvidia-persistenced.service")
		if exitCode == 0 {
			fmt.Println("Successfully enabled nvidia-persistenced.service")
		} else {
			LogError("An error occurred while enabling nvidia-persistenced.service")
		}
	}

	plan, err := BuildTransactionPlan(targetMode, state, opts)
	if err != nil {
		LogError("Failed to build transaction plan: %v", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	createdFiles, err := ExecuteTransaction(plan)
	if err != nil {
		LogError("Transaction aborted: %v", err)
		os.Exit(1)
	}

	if err := RebuildInitramfs(ctx); err != nil {
		LogError("Initramfs build failed or was interrupted: %v", err)
		LogError("Triggering Emergency Rollback...")

		if rbErr := RollbackTransaction(createdFiles); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")

			RebuildInitramfs(context.Background())
		}
		os.Exit(1)
	}

	state.CurrentMode = targetMode
	if err := SaveState(state); err != nil {
		LogWarning("Mode switched successfully, but failed to save state file: %v", err)
	}

	fmt.Println("Operation completed successfully")
	fmt.Println("Please reboot your computer for changes to take effect!")
}

func ResetSystem() {
	fmt.Println("Reverting changes made by EnvyControl...")

	RestoreSddmXsetup()

	plan := TransactionPlan{
		ToRemove: []string{
			BlacklistPath, UdevIntegratedPath, UdevPmPath,
			XorgPath, ExtraXorgPath, ModesetPath,
			LightdmScriptPath, LightdmConfigPath,
			"/etc/X11/xorg.conf.d/90-nvidia.conf",
			"/lib/udev/rules.d/50-remove-nvidia.rules",
			"/lib/udev/rules.d/80-nvidia-pm.rules",
		},
		ToCreate: []FileConfig{},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	createdFiles, err := ExecuteTransaction(plan)
	if err != nil {
		LogError("Reset transaction failed: %v", err)
		os.Exit(1)
	}

	os.Remove(StateFilePath)

	if err := RebuildInitramfs(ctx); err != nil {
		LogError("Initramfs rebuild failed or was interrupted: %v", err)
		LogError("Triggering Emergency Rollback...")

		if rbErr := RollbackTransaction(createdFiles); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")
			RebuildInitramfs(context.Background())
		}
		os.Exit(1)
	}

	fmt.Println("Operation completed successfully")
}

func ResetSddm() {
	fmt.Println("Restoring default Xsetup file...")
	plan := TransactionPlan{
		ToRemove: []string{},
		ToCreate: []FileConfig{{Path: SddmXsetupPath, Content: SddmXsetupContent, Executable: true}},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	_, err := ExecuteTransaction(plan)
	if err != nil {

		if ctx.Err() == context.Canceled {
			LogError("Reset SDDM was interrupted by user.")
		} else {
			LogError("Reset SDDM failed: %v", err)
		}
		os.Exit(1)
	}

	fmt.Println("Operation completed successfully")
}
