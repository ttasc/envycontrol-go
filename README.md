# 👁‍🗨 EnvyControl

[![CI](https://github.com/ttasc/envycontrol-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ttasc/envycontrol-go/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ttasc/envycontrol-go?color=success)](https://github.com/ttasc/envycontrol-go/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ttasc/envycontrol-go)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/ttasc/envycontrol-go)](https://goreportcard.com/report/github.com/ttasc/envycontrol-go)
[![Platform](https://img.shields.io/badge/Platform-Linux-blue)](https://www.kernel.org/)
[![License: MIT](https://img.shields.io/github/license/ttasc/envycontrol-go)](https://github.com/ttasc/envycontrol-go/blob/main/LICENSE)

> **A minimalist, fail-safe CLI tool for GPU mode switching and power management on Nvidia Optimus Linux laptops.**

---

## 🏛️ Tribute & Core Philosophy

**The Story:**
EnvyControl was originally created in 2021 by [Victor (bayasdev)](https://github.com/bayasdev) as a Python script. It became a beloved staple in the Linux community for taming Nvidia Optimus laptops. When putting the original project into maintenance mode, Victor wrote: *"If I built this today, I'd probably use Go or Rust :)"*

This project is the realization of that thought. We owe a massive debt of gratitude to Victor for the incredible foundation, research, and community he built.

**The Philosophy: "Do one thing and do it well"**
As Linux desktop environments rapidly evolve (especially with the widespread adoption of Wayland), automatically patching Display Managers and hacking X11 configurations has become fragile and prone to breaking user systems.

This Go rewrite takes a strict Unix philosophy approach: **EnvyControl now ONLY handles PCIe hardware routing, kernel module blacklisting, systemd daemon states, and dynamic power management.** It does not touch your Display Manager, and it does not write custom X11 overclocking files. It prepares the hardware perfectly, leaving UI-layer configurations entirely up to you.

---

## ✨ Why the Go Rewrite? (What's New)

Rebuilding this tool in Go allowed us to introduce enterprise-grade safety mechanisms and modern display server support:

*   🛡️ **Atomic Transactions & Rollback:** File operations are no longer sequential writes. Changes are staged and applied atomically. If an error occurs (e.g., out of disk space), an emergency rollback is triggered automatically, ensuring your system is never left in a broken, unbootable state.
*   🧱 **Signal Shielding (Anti-Brick Protection):** Interrupting the critical `initramfs` rebuild phase (like accidentally pressing `Ctrl+C`) can brick your boot image. EnvyControl now actively intercepts and blocks termination signals during this phase.
*   🧠 **Stateless Architecture:** The old Python version relied on a `cache.json` file to remember your PCI Bus ID. This rewrite drops the cache entirely. It probes your hardware state directly from the Linux Kernel SysFS (`/sys/bus/pci/`) in real-time.
*   🚀 **Wayland Native Support:** Built-in `--wayland` flag seamlessly configures kernel modesetting, enables `fbdev`, ensures VRAM persistence, and correctly hooks into Nvidia's sleep/resume systemd services to prevent crashes on modern GNOME/KDE environments.
*   📦 **Zero Dependencies:** Distributed as a single, statically linked binary. This completely bypasses Python's `PEP668` restrictions, eliminating the annoying `pip install` breakages on modern Debian and Ubuntu releases.

---

## 📖 Graphics Modes Explained

EnvyControl supports three distinct operational modes:

### 1. `integrated`
*   **How it works:** Completely powers off the Nvidia dGPU by blacklisting modules and removing the card from the PCI bus via Udev rules.
*   **Pros:** Maximum battery life. Zero fan noise. Flawless Wayland experience.
*   **Cons:** You cannot use external monitors if your laptop's HDMI/DisplayPort is physically wired to the Nvidia card.

### 2. `hybrid`
*   **How it works:** The default Windows-like behavior. Uses PRIME offloading. The dGPU sleeps and dynamically wakes up only when requested (RTD3).
*   **Pros:** Good balance of battery and on-demand performance.
*   **Cons:** External displays may lag or fail to work properly depending on your laptop's internal MUX switch and port wiring.

### 3. `nvidia`
*   **How it works:** Forces the Nvidia dGPU to stay awake and handle all rendering.
*   **Pros:** Maximum performance. Required for reliable external monitor usage on most laptops. Best for fixing screen tearing on legacy setups.
*   **Cons:** Terrible battery life. Your laptop will run hotter. *(Note: Requires the `--wayland` flag on modern distros to avoid falling back to X11)*.

---

## ⬇️ Installation

### Pre-compiled Binary (Recommended)
Download the [latest](https://github.com/ttasc/envycontrol-go/releases) static binary, make it executable, and move it to your system path.

### Build from Source
If you have the Go toolchain installed:

```bash
git clone https://github.com/ttasc/envycontrol-go.git
cd envycontrol-go
make build
sudo make install
```

## ⚙️ Configurations Guide (X11 Only)

> [!TIP]
> **Using Wayland?** If you use the `--wayland` flag when switching modes, you can safely skip this entire section! Modern compositors like Mutter (GNOME) and KWin (KDE) will route the hardware correctly out of the box using DRM without needing custom scripts or configs.

> [!NOTE]
> Because EnvyControl now strictly focuses on hardware states, it **does not** configure your Display Manager or X11 tearing fixes automatically. If you require these features in `nvidia` mode on legacy X11 sessions, here is how to apply them manually.

### 1. Display Manager Setup (Required for X11 `nvidia` mode)
To ensure your Display Manager routes the screen correctly when exclusively using the Nvidia GPU, you need to run this `xrandr` script upon login:

```bash
#!/bin/sh
[ "$(envycontrol -q)" = "nvidia" ] && xrandr --setprovideroutputsource modesetting NVIDIA-0
xrandr --auto
```

**Where to put it:**
*   **SDDM (KDE):** Append the script to `/usr/share/sddm/scripts/Xsetup`.
*   **LightDM:** Save the script to `/etc/lightdm/nvidia.sh`, make it executable, and add `display-setup-script=/etc/lightdm/nvidia.sh` to your `/etc/lightdm/lightdm.conf` under the `[Seat:*]` section.
*   **startx / Window Managers:** Simply add the commands to your `~/.xinitrc` before executing your WM.

### 2. X11 Hacks (Overclocking & Tearing Fixes)
If you experience screen tearing on X11 or want to enable GPU overclocking (Coolbits), create a custom Xorg configuration file:

```bash
sudo nano /etc/X11/xorg.conf.d/20-nvidia-hacks.conf
```

Paste the following block (adjust `Coolbits` value as needed for your specific use case):

```text
Section "OutputClass"
    Identifier "nvidia-hacks"
    MatchDriver "nvidia-drm"
    Driver "nvidia"
    Option "ForceCompositionPipeline" "true"
    Option "Coolbits" "28"
EndSection
```

---

### 🗑️ Uninstallation

Do **not** just delete the binary manually. The system needs to revert the configurations and safely rebuild your boot image (`initramfs`) to avoid a black screen on the next boot.

To safely uninstall, simply run:
```bash
# This will safely remove all configs, restore systemd services, and rebuild your initramfs
sudo envycontrol --reset

# Then remove the binary and backup data
sudo rm /usr/local/bin/envycontrol
sudo rm -rf /var/lib/envycontrol
```
*(If you kept the source code folder, you can also just run `sudo make uninstall` which automates these steps).*

<details>
  <summary> Or you can manually remove them (click to expand)</summary>

  ```bash
  # 1. System configuration files:
  /etc/modprobe.d/blacklist-nvidia.conf
  /etc/udev/rules.d/50-remove-nvidia.rules
  /etc/udev/rules.d/80-nvidia-pm.rules
  /etc/X11/xorg.conf
  /etc/modprobe.d/nvidia.conf

  # 2. EnvyControl's own storage directory:
  /var/lib/envycontrol/ # (Contains the Transaction Engine backup.tar.gz file).

  # 3. Disable Nvidia suspend services:
  sudo systemctl disable nvidia-persistenced nvidia-suspend nvidia-resume nvidia-hibernate
  ```
</details>

---

## ⚡️ Usage

Check your current active mode:
```bash
envycontrol --query
```

Switch to Integrated mode:
```bash
sudo envycontrol -s integrated
```

Switch to Hybrid mode and enable fine-grained RTD3 power control (default level 2):
```bash
sudo envycontrol -s hybrid --rtd3
```

Switch to Nvidia mode **(Optimized for Wayland)**:
```bash
sudo envycontrol -s nvidia --wayland
```

Switch to Nvidia mode **(Legacy X11)**:
```bash
sudo envycontrol -s nvidia
```

Update EnvyControl to the latest version:
```bash
sudo envycontrol --update
```

### The `NV_MODULE` Environment Variable
If your distribution uses a non-standard Nvidia kernel module name (e.g., Debian sometimes uses `nvidia-current`), you can pass the `NV_MODULE` environment variable instead of relying on legacy flags:

```bash
sudo NV_MODULE="nvidia-current" envycontrol -s nvidia --wayland
```

---

## 🚑 Troubleshooting & FAQ

- **Ubuntu Conflict (gpu-manager)**</br>
Ubuntu bundles its own Optimus tool which fights with EnvyControl. Disable it before switching modes:
  ```bash
  sudo prime-select on-demand
  sudo systemctl mask gpu-manager.service
  ```

- **Debian Black Screen on Nvidia Mode (X11)**</br>
If you face a black screen upon login on X11, Debian might need an explicit screen refresh. Add this to your user session:
  ```bash
  echo "xrandr --auto" >> ~/.xsessionrc
  ```

- **Display runs at 1 FPS when lid is closed on Hybrid Mode**</br>
This is a known bug with the proprietary Nvidia Linux drivers and Xorg PRIME implementation. There is no magic fix from EnvyControl's side. You can try disabling DRI3 by adding `LIBGL_DRI3_DISABLE=true` to your `/etc/environment`, but mileage may vary.

- **How do I completely uninstall and revert to defaults?**</br>
EnvyControl features a built-in total reset command. This safely removes all generated Udev, Modprobe, and Xorg configurations, resets systemd sleep daemons, and rebuilds your initramfs to factory defaults.
  ```bash
  sudo envycontrol --reset
  ```

---

## 🤝 Contributing & License

EnvyControl is free and open-source software. Continuing the legacy of the original project, this rewrite is released under the [MIT License](LICENSE).

Found a bug? Have an idea? Pull Requests and Issues are highly welcome! Let's keep the Linux Optimus ecosystem thriving.
