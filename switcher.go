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

	// UX Print
	if targetMode == "hybrid" {
		rtd3Str := "False"
		if opts.Rtd3Value != nil {
			rtd3Str = fmt.Sprintf("%d", *opts.Rtd3Value)
		}
		fmt.Printf("Enable PCI-Express Runtime D3 (RTD3) Power Management: %s\n", rtd3Str)
	}

	state := LoadState()

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

	// Establish an OS signal shield to prevent incomplete executions during file I/O
	_, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Execute I/O safely
	createdFiles, err := ExecuteTransaction(plan)
	if err != nil {
		cancel()
		LogError("Transaction aborted: %v", err)
		os.Exit(1)
	}

	// POINT OF NO RETURN
	// Release the interrupt shield for the context. From now on, Go context won't be cancelled.
	cancel()

	// Set up a custom shield that simply ignores signals and warns the user
	ignoreChan := make(chan os.Signal, 1)
	signal.Notify(ignoreChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range ignoreChan {
			LogWarning("\n[LOCKED] Critical kernel operation in progress. Interruptions are ignored to prevent system brick. Please wait...")
		}
	}()

	// Rebuild initramfs natively. Because we pass context.Background() and the child process
	// has Setpgid: true, it cannot be interrupted by the user terminal.
	if err := RebuildInitramfs(context.Background()); err != nil {
		LogError("Initramfs build failed natively: %v", err)
		LogError("Triggering Emergency Rollback to restore safe state...")

		if rbErr := RollbackTransaction(createdFiles); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")
			RebuildInitramfs(context.Background())
		}
		os.Exit(1)
	}

	signal.Stop(ignoreChan)

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

	// A plan with empty ToCreate forces a safe, fully-backed-up deletion
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

	// Establish an OS signal shield
	_, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	createdFiles, err := ExecuteTransaction(plan)
	if err != nil {
		cancel()
		LogError("Reset transaction failed: %v", err)
		os.Exit(1)
	}

	// POINT OF NO RETURN
	cancel()

	ignoreChan := make(chan os.Signal, 1)
	signal.Notify(ignoreChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range ignoreChan {
			LogWarning("\n[LOCKED] Critical kernel operation in progress. Interruptions are ignored to prevent system brick. Please wait...")
		}
	}()

	os.Remove(StateFilePath)

	if err := RebuildInitramfs(context.Background()); err != nil {
		LogError("Initramfs build failed natively: %v", err)
		LogError("Triggering Emergency Rollback to restore safe state...")

		if rbErr := RollbackTransaction(createdFiles); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")
			RebuildInitramfs(context.Background())
		}
		os.Exit(1)
	}

	signal.Stop(ignoreChan)

	fmt.Println("Operation completed successfully")
}
