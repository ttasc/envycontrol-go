package main

import (
	"fmt"
	"os"
	"os/exec"
)

// SwitchOptions chứa toàn bộ các cờ tùy chọn khi switch mode
type SwitchOptions struct {
	DisplayManager   string
	ForceComp        bool
	CoolbitsValue    *int // Dùng pointer để phân biệt giữa việc "có truyền giá trị" và "không truyền"
	Rtd3Value        *int
	UseNvidiaCurrent bool
}

// RebuildInitramfs gọi lệnh update initramfs tương ứng với distro hiện tại
func RebuildInitramfs() {
	var command []string

	// Dùng if/else if để check sự tồn tại của file cấu hình đặc trưng của từng Distro
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
		// Không detect được Distro
		command = []string{}
	}

	// Nếu hệ thống có systemd-inhibit, wrap lệnh lại để chống sleep/shutdown trong lúc build
	_, err := exec.LookPath("systemd-inhibit")
	if err == nil && len(command) > 0 {
		wrapped := []string{
			"systemd-inhibit",
			"--who=envycontrol",
			"--why", "Rebuilding initramfs",
			"--",
		}
		command = append(wrapped, command...)
	}

	if len(command) > 0 {
		fmt.Println("Rebuilding the initramfs...")
		// Tham số quiet = !Verbose (Nếu Verbose -> in log ra stdout/stderr, nếu không -> DevNull)
		exitCode, err := RunCommand(!Verbose, command[0], command[1:]...)
		if exitCode == 0 && err == nil {
			fmt.Println("Successfully rebuilt the initramfs!")
		} else {
			LogError("An error ocurred while rebuilding the initramfs")
		}
	}
}

// Chuyển đổi trạng thái đồ họa (Trái tim của dự án)
func SwitchMode(mode string, opts SwitchOptions) {
	fmt.Printf("Switching to %s mode\n", mode)

	switch mode {
	case "integrated":
		// 1. Tắt service persistenced
		exitCode, _ := RunCommand(!Verbose, "systemctl", "disable", "nvidia-persistenced.service")
		if exitCode == 0 {
			fmt.Println("Successfully disabled nvidia-persistenced.service")
		} else {
			LogError("An error ocurred while disabling service")
		}

		// 2. Dọn rác config cũ
		Cleanup()

		// 3. Tạo rule giết card Nvidia
		CreateFile(BlacklistPath, BlacklistContent, false)
		CreateFile(UdevIntegratedPath, UdevIntegrated, false)

		// 4. Build lại kernel ramdisk
		RebuildInitramfs()

	case "hybrid":
		rtd3Str := "False"
		if opts.Rtd3Value != nil {
			rtd3Str = fmt.Sprintf("%d", *opts.Rtd3Value)
		}
		fmt.Printf("Enable PCI-Express Runtime D3 (RTD3) Power Management: %s\n", rtd3Str)

		Cleanup()

		exitCode, _ := RunCommand(!Verbose, "systemctl", "enable", "nvidia-persistenced.service")
		if exitCode == 0 {
			fmt.Println("Successfully enabled nvidia-persistenced.service")
		} else {
			LogError("An error ocurred while enabling service")
		}

		if opts.Rtd3Value == nil {
			if opts.UseNvidiaCurrent {
				CreateFile(ModesetPath, ModesetCurrentContent, false)
			} else {
				CreateFile(ModesetPath, ModesetContent, false)
			}
		} else {
			// Thiết lập modprobe RTD3
			if opts.UseNvidiaCurrent {
				CreateFile(ModesetPath, fmt.Sprintf(ModesetCurrentRtd3, *opts.Rtd3Value), false)
			} else {
				CreateFile(ModesetPath, fmt.Sprintf(ModesetRtd3, *opts.Rtd3Value), false)
			}
			CreateFile(UdevPmPath, UdevPmContent, false)
		}

		RebuildInitramfs()

	case "nvidia":
		coolbitsStr := "False"
		if opts.CoolbitsValue != nil {
			coolbitsStr = fmt.Sprintf("%d", *opts.CoolbitsValue)
		}
		fmt.Printf("Enable ForceCompositionPipeline: %t\n", opts.ForceComp)
		fmt.Printf("Enable Coolbits: %s\n", coolbitsStr)

		exitCode, _ := RunCommand(!Verbose, "systemctl", "enable", "nvidia-persistenced.service")
		if exitCode == 0 {
			fmt.Println("Successfully enabled nvidia-persistenced.service")
		} else {
			LogError("An error ocurred while enabling service")
		}

		Cleanup()

		// Đọc Cache an toàn thay cho thủ thuật ContextManager của Python
		nvidiaGpuPciBus := ReadPciBusWithCache()
		igpuVendor := GetIgpuVendor()

		if igpuVendor == "intel" {
			CreateFile(XorgPath, fmt.Sprintf(XorgIntel, nvidiaGpuPciBus), false)
		} else if igpuVendor == "amd" {
			CreateFile(XorgPath, fmt.Sprintf(XorgAmd, nvidiaGpuPciBus), false)
		}

		if opts.UseNvidiaCurrent {
			CreateFile(ModesetPath, ModesetCurrentContent, false)
		} else {
			CreateFile(ModesetPath, ModesetContent, false)
		}

		// Xây dựng Extra Xorg config bằng cách nối chuỗi
		if opts.ForceComp || opts.CoolbitsValue != nil {
			extraConfig := ExtraXorgContent
			if opts.ForceComp {
				extraConfig += ForceComp
			}
			if opts.CoolbitsValue != nil {
				extraConfig += fmt.Sprintf(Coolbits, *opts.CoolbitsValue)
			}
			extraConfig += "EndSection\n"
			CreateFile(ExtraXorgPath, extraConfig, false)
		}

		// Thiết lập Display Manager (Hack Xrandr)
		dm := opts.DisplayManager
		if dm == "" {
			dm = GetDisplayManager()
		}

		if dm == "sddm" {
			// Backup file Xsetup của sddm nếu nó đã tồn tại
			if fileExists(SddmXsetupPath) {
				LogInfo("Creating Xsetup backup")
				content, _ := os.ReadFile(SddmXsetupPath)
				CreateFile(SddmXsetupPath+".bak", string(content), false)
			}
			CreateFile(SddmXsetupPath, GenerateXrandrScript(igpuVendor), true) // File script cần +x (executable = true)
		} else if dm == "lightdm" {
			CreateFile(LightdmScriptPath, GenerateXrandrScript(igpuVendor), true)
			CreateFile(LightdmConfigPath, LightdmConfigContent, false)
		}

		RebuildInitramfs()
	}

	fmt.Println("Operation completed successfully")
	fmt.Println("Please reboot your computer for changes to take effect!")
}
