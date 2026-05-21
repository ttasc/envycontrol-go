package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// SwitchMode is the primary orchestrator for transitioning the system between graphics modes.
// It coordinates state retrieval, daemon management, plan building, transaction execution,
// and safe kernel initramfs rebuilding.
func SwitchMode(targetMode string, opts SwitchOptions) {
	fmt.Printf("Switching to %s mode\n", targetMode)

	state := LoadState()

	// Pre-flight: Handle persistent SDDM backups
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

	// Toggle the persistence daemon
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

	// Build the execution plan
	plan, err := BuildTransactionPlan(targetMode, state, opts)
	if err != nil {
		LogError("Failed to build transaction plan: %v", err)
		os.Exit(1)
	}

	// Establish an OS signal shield to prevent incomplete executions
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Execute I/O safely
	createdFiles, err := ExecuteTransaction(plan)
	if err != nil {
		LogError("Transaction aborted: %v", err)
		os.Exit(1)
	}

	// Rebuild initramfs with graceful cancellation awareness
	if err := RebuildInitramfs(ctx); err != nil {
		LogError("Initramfs build failed or was interrupted: %v", err)
		LogError("Triggering Emergency Rollback...")

		if rbErr := RollbackTransaction(createdFiles); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")

			// Critical: Must use a fresh Background context because the previous one
			// may have been cancelled by user interruption.
			RebuildInitramfs(context.Background())
		}
		os.Exit(1)
	}

	// Post-flight: Save updated state
	state.CurrentMode = targetMode
	if err := SaveState(state); err != nil {
		LogWarning("Mode switched successfully, but failed to save state file: %v", err)
	}

	fmt.Println("Operation completed successfully")
	fmt.Println("Please reboot your computer for changes to take effect!")
}

// ResetSystem purges all configurations applied by the tool and restores
// the system to the Linux distribution's vanilla state.
func ResetSystem() {
	fmt.Println("Reverting changes made by EnvyControl...")

	RestoreSddmXsetup()

	// A plan with empty ToCreate forces a safe, fully-backed-up deletion
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

// ResetSddm isolates the recovery of the SDDM Xsetup script.
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
