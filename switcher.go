package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// SwitchMode is the primary orchestrator for transitioning the system between graphics modes.
// It coordinates real-time hardware probing, daemon management, plan building,
// atomic transaction execution, and safe kernel initramfs rebuilding.
func SwitchMode(targetMode string, opts SwitchOptions) {
	LogInfo("Switching to %s mode\n", targetMode)

	// Print UX feedback for power management configurations
	if targetMode == "hybrid" {
		rtd3Str := "False"
		if opts.Rtd3Value != nil {
			rtd3Str = fmt.Sprintf("%d", *opts.Rtd3Value)
		}
		LogInfo("Enable PCI-Express Runtime D3 (RTD3) Power Management: %s\n", rtd3Str)
	}

	// In the stateless architecture, we must dynamically probe the SysFS to find
	// the Nvidia PCI Bus ID only when transitioning to 'nvidia' mode.
	var pciBus, igpuVendor string
	if targetMode == "nvidia" {
		var err error
		pciBus, err = ProbeNvidiaPciBus()
		if err != nil {
			LogError("Nvidia GPU not found on PCI bus. It might be physically powered off (Integrated mode).")
			LogError("Please switch to 'hybrid' mode and reboot first to wake it up.")
			os.Exit(1)
		}
		igpuVendor = ProbeIgpuVendor()
	}

	// nvidia-persistenced keeps the kernel module loaded to prevent driver teardown latency.
	// We disable it in integrated mode so the card can be powered off.
	ctxBg := context.Background()
	if targetMode == "integrated" {
		exitCode, _ := RunCommand(ctxBg, !Verbose, "systemctl", "disable", "nvidia-persistenced.service")
		if exitCode == 0 {
			LogInfo("Successfully disabled nvidia-persistenced.service")
		}
	} else {
		exitCode, _ := RunCommand(ctxBg, !Verbose, "systemctl", "enable", "nvidia-persistenced.service")
		if exitCode == 0 {
			LogInfo("Successfully enabled nvidia-persistenced.service")
		}
	}

	// Calculate the necessary filesystem changes without touching the disk.
	plan, err := BuildTransactionPlan(targetMode, pciBus, igpuVendor, opts)
	if err != nil {
		LogError("Failed to build transaction plan: %v", err)
		os.Exit(1)
	}

	// Establish an OS signal shield. If the user hits Ctrl+C during file I/O,
	// the transaction engine will catch it, abort cleanly, and rollback.
	_, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	createdFiles, err := ExecuteTransaction(plan)
	if err != nil {
		cancel() // Manually release the context
		LogError("Transaction aborted: %v", err)
		os.Exit(1)
	}

	// =========================================================================
	// POINT OF NO RETURN
	// From this point onward, the filesystem is modified. We must rebuild the
	// initramfs to match. Interrupting this process will brick the boot sequence.
	// =========================================================================

	// Release the interrupt shield for the context.
	cancel()

	// Set up a custom shield that actively traps and ignores signals, warning the user.
	ignoreChan := make(chan os.Signal, 1)
	signal.Notify(ignoreChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range ignoreChan {
			LogWarning("\n[LOCKED] Critical kernel operation in progress. Interruptions are ignored to prevent system brick. Please wait...")
		}
	}()

	// Execute the distro-specific initramfs builder. We pass a fresh Background context
	// so the child process cannot be cancelled by earlier signals.
	if err := RebuildInitramfs(context.Background()); err != nil {
		LogError("Initramfs build failed natively: %v", err)
		LogError("Triggering Emergency Rollback to restore safe state...")

		// If initramfs generation fails (e.g., disk full), revert the files
		if rbErr := RollbackTransaction(createdFiles); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")
			// Best-effort attempt to sync the kernel image back to the safe config
			if rebuildErr := RebuildInitramfs(context.Background()); rebuildErr != nil {
				LogWarning("Fallback initramfs rebuild also failed: %v", rebuildErr)
			}
		}
		os.Exit(1)
	}

	// Safely release the custom signal trap
	signal.Stop(ignoreChan)

	LogInfo("Operation completed successfully")
	LogInfo("Please reboot your computer for changes to take effect!")
}

// ResetSystem safely removes all configuration files managed by the application.
// It forces a clean slate and regenerates the initramfs to restore the
// Linux distribution's vanilla graphical state.
func ResetSystem() {
	LogInfo("Reverting changes made by EnvyControl...")

	// A plan with an empty ToCreate list forces the Transaction Engine
	// to perform a safely-backed-up deletion of all managed paths.
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

	// Set up interruptible context for File I/O
	_, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	createdFiles, err := ExecuteTransaction(plan)
	if err != nil {
		cancel()
		LogError("Reset transaction failed: %v", err)
		os.Exit(1)
	}

	// Release context, lock the process for kernel operations
	cancel()

	ignoreChan := make(chan os.Signal, 1)
	signal.Notify(ignoreChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range ignoreChan {
			LogWarning("\n[LOCKED] Critical kernel operation in progress. Interruptions are ignored to prevent system brick. Please wait...")
		}
	}()

	// Rebuild initramfs to remove Nvidia configurations from the boot image
	if err := RebuildInitramfs(context.Background()); err != nil {
		LogError("Initramfs build failed natively: %v", err)
		LogError("Triggering Emergency Rollback to restore safe state...")

		if rbErr := RollbackTransaction(createdFiles); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")
			if rebuildErr := RebuildInitramfs(context.Background()); rebuildErr != nil {
				LogWarning("Fallback initramfs rebuild also failed: %v", rebuildErr)
			}
		}
		os.Exit(1)
	}

	signal.Stop(ignoreChan)
	LogInfo("Operation completed successfully")
}
