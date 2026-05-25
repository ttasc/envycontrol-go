package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Verbose controls whether debug logs and command outputs are printed.
var Verbose bool

// For mocking while testing parseOSRelease
var OsReleasePath = "/etc/os-release"

// --- Logging Utilities ---

// LogInfo prints a standard informational message.
func LogInfo(format string, a ...any) {
	fmt.Printf("INFO: "+format+"\n", a...)
}

// LogWarning prints a non-fatal warning message.
func LogWarning(format string, a ...any) {
	fmt.Printf("WARNING: "+format+"\n", a...)
}

// LogError prints an error message.
func LogError(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", a...)
}

// LogDebug prints a debug message if the Verbose flag is enabled.
func LogDebug(format string, a ...any) {
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

	// Isolate the child process from the terminal's process group.
	// This prevents terminal signals (like Ctrl+C/SIGINT) from reaching the child directly.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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

// parseOSRelease parses /etc/os-release to identify the Linux distribution safely.
func parseOSRelease() (id string, idLike string) {
	data, err := os.ReadFile(OsReleasePath)
	if err != nil {
		data, err = os.ReadFile("/usr/lib/os-release") // Fallback
		if err != nil {
			return "", ""
		}
	}

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(line[3:], `"'`)
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			idLike = strings.Trim(line[8:], `"'`)
		}
	}
	return id, idLike
}

// RebuildInitramfs determines the current Linux distribution and invokes the
// correct tool to regenerate the initramfs. It respects Context cancellation.
func RebuildInitramfs(ctx context.Context) error {
	fileExists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

	id, idLike := parseOSRelease()
	command := getInitramfsCommand(id, idLike, fileExists)

	if len(command) == 0 {
		LogWarning("Unsupported distribution (ID: %s). Could not determine initramfs builder.", id)
		LogWarning("Skipping initramfs rebuild. You may need to update your boot image manually.")
		return nil
	}

	// Wrap with systemd-inhibit to prevent sleep/shutdown during critical build
	if _, err := exec.LookPath("systemd-inhibit"); err == nil {
		wrapped := []string{"systemd-inhibit", "--who=envycontrol", "--why", "Rebuilding initramfs", "--"}
		command = append(wrapped, command...)
	}

	LogInfo("Rebuilding the initramfs. DO NOT TURN OFF YOUR COMPUTER...")

	exitCode, err := RunCommand(ctx, !Verbose, command[0], command[1:]...)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("initramfs rebuild was interrupted by user/system")
		}
		return fmt.Errorf("initramfs command failed with exit code %d: %w", exitCode, err)
	}

	LogInfo("Successfully rebuilt the initramfs!")
	return nil
}

func getInitramfsCommand(id, idLike string, fileExists func(string) bool) []string {
	var command []string

	if fileExists("/ostree") || fileExists("/sysroot/ostree") {
		return []string{"rpm-ostree", "initramfs", "--enable", "--arg=--force"}
	}

	osList := id + " " + idLike

	if strings.Contains(osList, "debian") || strings.Contains(osList, "ubuntu") {
		command = []string{"update-initramfs", "-u", "-k", "all"}
	} else if strings.Contains(osList, "fedora") || strings.Contains(osList, "rhel") || strings.Contains(osList, "centos") || strings.Contains(osList, "suse") {
		command = []string{"dracut", "--force", "--regenerate-all"}
	} else if strings.Contains(osList, "arch") {
		if id == "endeavouros" && fileExists("/usr/bin/dracut") {
			command = []string{"dracut-rebuild"}
		} else {
			command = []string{"mkinitcpio", "-P"}
		}
	} else if strings.Contains(osList, "altlinux") {
		command = []string{"make-initrd"}
	}

	return command
}
