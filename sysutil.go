package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Verbose controls whether debug logs and command outputs are printed.
var Verbose bool

// --- Logging Utilities ---

// LogInfo prints a standard informational message.
func LogInfo(format string, a ...interface{}) {
	fmt.Printf("INFO: "+format+"\n", a...)
}

// LogWarning prints a non-fatal warning message.
func LogWarning(format string, a ...interface{}) {
	fmt.Printf("WARNING: "+format+"\n", a...)
}

// LogError prints an error message.
func LogError(format string, a ...interface{}) {
	fmt.Printf("ERROR: "+format+"\n", a...)
}

// LogDebug prints a debug message if the Verbose flag is enabled.
func LogDebug(format string, a ...interface{}) {
	if Verbose {
		fmt.Printf("DEBUG: "+format+"\n", a...)
	}
}

// --- System Utilities ---

// AssertRoot ensures the program is running with root privileges, exiting if not.
func AssertRoot() {
	if os.Geteuid() != 0 {
		LogError("This operation requires root privileges")
		os.Exit(1)
	}
}

// RunCommand safely wraps os/exec.
// If quiet is true and Verbose is false, all output is silenced.
// If interrupted by context cancellation, it gracefully kills the child process.
func RunCommand(ctx context.Context, quiet bool, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	if Verbose && !quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else if quiet && !Verbose {
		cmd.Stdout = nil
		cmd.Stderr = nil
	} else {
		// Buffer output to hide it but retain it if needed internally
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
	}

	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode(), err
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// RebuildInitramfs determines the current Linux distribution and invokes the
// correct tool to regenerate the initramfs. It respects Context cancellation.
func RebuildInitramfs(ctx context.Context) error {
	var command []string

	fileExists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	// Detect appropriate initramfs builder
	if fileExists("/ostree") || fileExists("/sysroot/ostree") {
		fmt.Println("Rebuilding the initramfs with rpm-ostree...")
		command = []string{"rpm-ostree", "initramfs", "--enable", "--arg=--force"}
	} else if fileExists("/etc/debian_version") {
		command = []string{"update-initramfs", "-u", "-k", "all"}
	} else if fileExists("/etc/redhat-release") || fileExists("/usr/bin/zypper") {
		command = []string{"dracut", "--force", "--regenerate-all"}
	} else if fileExists("/usr/lib/endeavouros-release") && fileExists("/usr/bin/dracut") {
		command = []string{"dracut-rebuild"}
	} else if fileExists("/etc/altlinux-release") {
		command = []string{"make-initrd"}
	} else if fileExists("/etc/arch-release") {
		command = []string{"mkinitcpio", "-P"}
	} else {
		LogWarning("Unsupported distribution: could not determine initramfs builder.")
		LogWarning("Skipping initramfs rebuild. You may need to update your boot image manually.")
		return nil
	}

	// Wrap with systemd-inhibit to prevent sleep/shutdown during critical build
	if _, err := exec.LookPath("systemd-inhibit"); err == nil {
		wrapped := []string{
			"systemd-inhibit",
			"--who=envycontrol",
			"--why", "Rebuilding initramfs",
			"--",
		}
		command = append(wrapped, command...)
	}

	fmt.Println("Rebuilding the initramfs. DO NOT TURN OFF YOUR COMPUTER...")

	exitCode, err := RunCommand(ctx, !Verbose, command[0], command[1:]...)
	if err != nil {
		// Identify if the failure was due to user/system interruption
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("initramfs rebuild was interrupted by user/system")
		}
		return fmt.Errorf("initramfs command failed with exit code %d: %v", exitCode, err)
	}

	fmt.Println("Successfully rebuilt the initramfs!")
	return nil
}

// BackupSddmXsetup creates a persistent backup of the original SDDM Xsetup script.
// It will never overwrite an existing backup, ensuring the vanilla file is protected.
func BackupSddmXsetup() {
	bakPath := SddmXsetupPath + ".bak"

	// Skip if original file doesn't exist
	if _, err := os.Stat(SddmXsetupPath); os.IsNotExist(err) {
		return
	}

	// Skip if backup already exists to prevent overwriting with a tampered file
	if _, err := os.Stat(bakPath); err == nil {
		return
	}

	if content, err := os.ReadFile(SddmXsetupPath); err == nil {
		if err := os.WriteFile(bakPath, content, 0755); err == nil {
			LogDebug("Created persistent backup for SDDM Xsetup at %s", bakPath)
		} else {
			LogWarning("Failed to create SDDM backup: %v", err)
		}
	}
}

// RestoreSddmXsetup restores the vanilla Xsetup file from the persistent backup,
// cleaning up the backup file afterward.
func RestoreSddmXsetup() {
	bakPath := SddmXsetupPath + ".bak"

	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return
	}

	if content, err := os.ReadFile(bakPath); err == nil {
		if err := os.WriteFile(SddmXsetupPath, content, 0755); err == nil {
			os.Remove(bakPath)
			LogInfo("Restored original SDDM Xsetup file")
		} else {
			LogError("Failed to restore SDDM Xsetup file: %v", err)
		}
	}
}
