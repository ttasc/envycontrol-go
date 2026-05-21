package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

var Verbose bool // Cờ cắm từ CLI

// --- Hệ thống Logging ---
func LogInfo(format string, a ...interface{}) {
	fmt.Printf("INFO: "+format+"\n", a...)
}
func LogWarning(format string, a ...interface{}) {
	fmt.Printf("WARNING: "+format+"\n", a...)
}
func LogError(format string, a ...interface{}) {
	fmt.Printf("ERROR: "+format+"\n", a...)
}
func LogDebug(format string, a ...interface{}) {
	if Verbose {
		fmt.Printf("DEBUG: "+format+"\n", a...)
	}
}

// Kiểm tra quyền Root
func AssertRoot() {
	if os.Geteuid() != 0 {
		LogError("This operation requires root privileges")
		os.Exit(1)
	}
}

// RunCommand bọc os/exec an toàn, hỗ trợ ngắt tiến trình bằng Context
func RunCommand(ctx context.Context, quiet bool, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	if Verbose && !quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else if quiet && !Verbose {
		cmd.Stdout = nil
		cmd.Stderr = nil
	} else {
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

// Lệnh build initramfs. Bắt lỗi Context Canceled nếu bị user ngắt.
func RebuildInitramfs(ctx context.Context) error {
	var command []string

	fileExists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}

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

	_, err := exec.LookPath("systemd-inhibit")
	if err == nil {
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
		// Nhận diện lỗi do Signal Interruption
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("initramfs rebuild was interrupted by user/system")
		}
		return fmt.Errorf("initramfs command failed with exit code %d: %v", exitCode, err)
	}

	fmt.Println("Successfully rebuilt the initramfs!")
	return nil
}

// BackupSddmXsetup tạo bản sao lưu vĩnh viễn cho Xsetup của SDDM.
func BackupSddmXsetup() {
	bakPath := SddmXsetupPath + ".bak"

	if _, err := os.Stat(SddmXsetupPath); os.IsNotExist(err) {
		return
	}
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

// RestoreSddmXsetup khôi phục file Xsetup gốc và dọn dẹp file .bak
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
