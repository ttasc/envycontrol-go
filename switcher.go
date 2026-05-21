package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Hàm hỗ trợ khóa tín hiệu ngắt để bảo vệ Critical Section
func protectCriticalSection() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	// Bắt SIGINT (Ctrl+C) và SIGTERM (Lệnh kill)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return sigChan
}

// SwitchMode là luồng điều phối chính để chuyển mode
func SwitchMode(targetMode string, opts SwitchOptions) {
	fmt.Printf("Switching to %s mode\n", targetMode)

	// 1. Tải trạng thái hiện tại (Đọc Source of Truth)
	state := LoadState()

	if targetMode == "integrated" || targetMode == "hybrid" {
		RestoreSddmXsetup() // Dọn dẹp sạch sẽ nếu không xài Nvidia
	} else if targetMode == "nvidia" {
		dm := opts.DisplayManager
		if dm == "" {
			dm = ProbeDisplayManager()
		}
		if dm == "sddm" {
			BackupSddmXsetup() // Chốt hạ bản backup trước khi Transaction ghi đè
		}
	}

	// 2. Bật/Tắt systemd service nvidia-persistenced
	if targetMode == "integrated" {
		exitCode, _ := RunCommand(!Verbose, "systemctl", "disable", "nvidia-persistenced.service")
		if exitCode == 0 {
			fmt.Println("Successfully disabled nvidia-persistenced.service")
		} else {
			LogError("An error occurred while disabling nvidia-persistenced.service")
		}
	} else {
		exitCode, _ := RunCommand(!Verbose, "systemctl", "enable", "nvidia-persistenced.service")
		if exitCode == 0 {
			fmt.Println("Successfully enabled nvidia-persistenced.service")
		} else {
			LogError("An error occurred while enabling nvidia-persistenced.service")
		}
	}

	// 3. Gọi Pure Builder tính toán Kế hoạch (Bản thiết kế)
	plan, err := BuildTransactionPlan(targetMode, state, opts)
	if err != nil {
		LogError("Failed to build transaction plan: %v", err)
		os.Exit(1)
	}

	// [BẢO VỆ BẰNG SIGNAL]: Thiết lập lá chắn ngắt
	sigChan := protectCriticalSection()
	go func() {
		<-sigChan
		LogError("\n[CRITICAL] Interrupted by user or system!")
		LogError("Triggering Emergency Rollback to prevent system brick...")
		RollbackTransaction()
		os.Exit(1) // Chỉ exit SAU KHI đã rollback xong
	}()

	// 4. Bàn giao bản thiết kế cho Transaction Engine (An toàn tuyệt đối)
	if err := ExecuteTransaction(plan); err != nil {
		LogError("Transaction aborted: %v", err)
		os.Exit(1)
	}

	// 5. Build lại Initramfs để áp dụng
	if err := RebuildInitramfs(); err != nil {
		LogError("Initramfs build failed: %v", err)
		LogError("Triggering Emergency Rollback...")

		if rbErr := RollbackTransaction(); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")
			// Best-effort để đồng bộ lại initramfs với file config vừa được cứu
			RebuildInitramfs()
		}
		os.Exit(1)
	}

	// Gỡ bỏ lá chắn an toàn khi đã hoàn tất
	signal.Stop(sigChan)

	// 6. Cập nhật State File SAU KHI mọi thứ đã thành công
	state.CurrentMode = targetMode
	if err := SaveState(state); err != nil {
		LogWarning("Mode switched successfully, but failed to save state file: %v", err)
	}

	fmt.Println("Operation completed successfully")
	fmt.Println("Please reboot your computer for changes to take effect!")
}

// ResetSystem khôi phục hệ thống về trạng thái ban đầu của Distro
func ResetSystem() {
	fmt.Println("Reverting changes made by EnvyControl...")

	RestoreSddmXsetup()

	// Dùng Transaction rỗng ToCreate để dọn dẹp an toàn có backup
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

	// [BẢO VỆ BẰNG SIGNAL]
	sigChan := protectCriticalSection()
	go func() {
		<-sigChan
		LogError("\n[CRITICAL] Interrupted by user or system!")
		LogError("Triggering Emergency Rollback to prevent system brick...")
		RollbackTransaction()
		os.Exit(1)
	}()

	if err := ExecuteTransaction(plan); err != nil {
		LogError("Reset transaction failed: %v", err)
		os.Exit(1)
	}

	os.Remove(StateFilePath)

	if err := RebuildInitramfs(); err != nil {
		LogError("Initramfs rebuild failed during reset: %v", err)
		LogError("Triggering Emergency Rollback...")

		if rbErr := RollbackTransaction(); rbErr != nil {
			LogError("CRITICAL: Rollback failed: %v", rbErr)
		} else {
			LogWarning("System configs safely rolled back.")
			LogWarning("Attempting to rebuild initramfs for the rolled-back state...")
			RebuildInitramfs()
		}
		os.Exit(1)
	}

	signal.Stop(sigChan)

	fmt.Println("Operation completed successfully")
}

// ResetSddm khôi phục file Xsetup mặc định
func ResetSddm() {
	fmt.Println("Restoring default Xsetup file...")
	plan := TransactionPlan{
		ToRemove: []string{},
		ToCreate: []FileConfig{{Path: SddmXsetupPath, Content: SddmXsetupContent, Executable: true}},
	}

	// [BẢO VỆ BẰNG SIGNAL]: Mặc dù không gọi initramfs nhưng File I/O vẫn cần an toàn
	sigChan := protectCriticalSection()
	go func() {
		<-sigChan
		LogError("\n[CRITICAL] Interrupted by user or system!")
		LogError("Triggering Emergency Rollback...")
		RollbackTransaction()
		os.Exit(1)
	}()

	if err := ExecuteTransaction(plan); err != nil {
		LogError("Reset SDDM failed: %v", err)
		os.Exit(1)
	}

	signal.Stop(sigChan)

	fmt.Println("Operation completed successfully")
}
