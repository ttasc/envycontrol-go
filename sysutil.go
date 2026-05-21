package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// Verbose flag (sẽ được set từ CLI args)
var Verbose bool

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

// --- Tiện ích OS ---

// Kiểm tra quyền root, exit nếu không phải root
func AssertRoot() {
	if os.Geteuid() != 0 {
		LogError("This operation requires root privileges")
		os.Exit(1)
	}
}

// Hàm ghi file. Tự động tạo thư mục cha nếu chưa có.
func CreateFile(path, content string, executable bool) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		LogError("Failed to create directory '%s': %v", dir, err)
		return
	}

	mode := fs.FileMode(0644)
	if executable {
		mode = 0755
	}

	err := os.WriteFile(path, []byte(content), mode)
	if err != nil {
		LogError("Failed to create file '%s': %v", path, err)
		return
	}

	if executable {
		if err := os.Chmod(path, 0755); err != nil {
			LogError("Failed to set execution privilege to file '%s': %v", path, err)
		} else {
			LogInfo("Added execution privilege to file %s", path)
		}
	}

	LogInfo("Created file %s", path)
	if Verbose {
		fmt.Print(content)
	}
}

// Xóa file rác và restore backup (bê y nguyên logic hàm cleanup python)
func Cleanup() {
	toRemove := []string{
		BlacklistPath,
		UdevIntegratedPath,
		UdevPmPath,
		XorgPath,
		ExtraXorgPath,
		ModesetPath,
		LightdmScriptPath,
		LightdmConfigPath,
		// legacy files
		"/etc/X11/xorg.conf.d/90-nvidia.conf",
		"/lib/udev/rules.d/50-remove-nvidia.rules",
		"/lib/udev/rules.d/80-nvidia-pm.rules",
	}

	for _, path := range toRemove {
		err := os.Remove(path)
		if err == nil {
			LogInfo("Removed file %s", path)
		} else if !os.IsNotExist(err) {
			// Chỉ log error nếu lỗi không phải là "file không tồn tại" (errno 2 trong Python)
			LogError("Failed to remove file '%s': %v", path, err)
		}
	}

	// Khôi phục Xsetup backup nếu có
	backupPath := SddmXsetupPath + ".bak"
	if _, err := os.Stat(backupPath); err == nil {
		LogInfo("Restoring Xsetup backup")
		content, readErr := os.ReadFile(backupPath)
		if readErr == nil {
			CreateFile(SddmXsetupPath, string(content), false)
			os.Remove(backupPath)
			LogInfo("Removed file %s", backupPath)
		}
	}
}

// RunCommand gọi bash/shell.
// Tham số 'quiet' tương đương stdout=subprocess.DEVNULL trong Python.
func RunCommand(quiet bool, name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)

	if Verbose && !quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else if quiet && !Verbose {
		cmd.Stdout = nil
		cmd.Stderr = nil
	} else {
		// Capture to internal buffer if we want to swallow output but return err
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
