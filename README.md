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

This Go rewrite takes a strict Unix philosophy approach: **EnvyControl now ONLY handles PCIe hardware routing, kernel module blacklisting, and RTD3 dynamic power management.** It does not touch your Display Manager, and it does not write custom X11 overclocking files. It prepares the hardware perfectly, leaving UI-layer configurations entirely up to you.

---

## ✨ Why the Go Rewrite? (What's New)

Rebuilding this tool in Go allowed us to introduce enterprise-grade safety mechanisms:

*   🛡️ **Atomic Transactions & Rollback:** File operations are no longer sequential writes. Changes are staged and applied atomically. If an error occurs (e.g., out of disk space), an emergency rollback is triggered automatically, ensuring your system is never left in a broken, unbootable state.
*   🧱 **Signal Shielding (Anti-Brick Protection):** Interrupting the critical `initramfs` rebuild phase (like accidentally pressing `Ctrl+C`) can brick your boot image. EnvyControl now actively intercepts and blocks termination signals during this phase.
*   🧠 **Stateless Architecture:** The old Python version relied on a `cache.json` file to remember your PCI Bus ID. This rewrite drops the cache entirely. It probes your hardware state directly from the Linux Kernel SysFS (`/sys/bus/pci/`) in real-time.
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
*   **Pros:** Maximum performance. Required for reliable external monitor usage on most laptops. Best for fixing X11 screen tearing.
*   **Cons:** Terrible battery life. Your laptop will run hotter. Wayland sessions may default back to X11 depending on your distro.

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

## ⚙️ Configurations Guide

> [!NOTE]
> Because EnvyControl now strictly focuses on hardware states, it **does not** configure your Display Manager or X11 tearing fixes. If you require these features in `nvidia` mode, here is how to apply them manually.

### 1. Display Manager Setup (Required for `nvidia` mode)
To ensure your Display Manager routes the screen correctly when exclusively using the Nvidia GPU, you need to run this `xrandr` script upon login:

```bash
#!/bin/sh
[ "$(envycontrol -q)" = "nvidia" ] && xrandr --setprovideroutputsource modesetting NVIDIA-0
xrandr --auto
```

**Where to put it:**
*   **SDDM (KDE):** Append the script to `/usr/share/sddm/scripts/Xsetup`.
*   **LightDM:** Save the script to `/etc/lightdm/nvidia.sh`, make it executable, and add `display-setup-script=/etc/lightdm/nvidia.sh` to your `/etc/lightdm/lightdm.conf` under the `[Seat:*]` section.
*   **GDM (GNOME):** GDM heavily favors Wayland now. If you force X11, you may need to place the script in `/etc/gdm3/Init/Default`. However, Wayland usually handles Nvidia routing natively without `xrandr` hacks.
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
# This will safely remove all configs and rebuild your initramfs
sudo envycontrol --reset

# Then remove the binary and backup data
sudo rm /usr/local/bin/envycontrol
sudo rm -rf /var/lib/envycontrol
```
*(If you kept the source code folder, you can also just run `sudo make uninstall` which automates these steps).*

> The below files are created by envycontrol, and you may want to remove them manually if they are not removed automatically to avoid any incorrect system behaviour:
> 1. System configuration files:
>   - `/etc/modprobe.d/blacklist-nvidia.conf`
>   - `/etc/udev/rules.d/50-remove-nvidia.rules`
>   - `/etc/udev/rules.d/80-nvidia-pm.rules`
>   - `/etc/X11/xorg.conf`
>   - `/etc/modprobe.d/nvidia.conf`
>
> 2. EnvyControl's own storage directory (Go version):
>   - `/var/lib/envycontrol/` *(Contains the Transaction Engine backup.tar.gz file).*

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

Switch to Nvidia mode:
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
sudo NV_MODULE="nvidia-current" envycontrol -s nvidia
```

---

## 🚑 Troubleshooting & FAQ

- **Ubuntu Conflict (gpu-manager)**</br>
Ubuntu bundles its own Optimus tool which fights with EnvyControl. Disable it before switching modes:
  ```bash
  sudo prime-select on-demand
  sudo systemctl mask gpu-manager.service
  ```

- **Debian Black Screen on Nvidia Mode**</br>
If you face a black screen upon login, Debian might need an explicit X11 screen refresh. Add this to your user session:
  ```bash
  echo "xrandr --auto" >> ~/.xsessionrc
  ```

- **Wayland session is missing on Gnome 43+**</br>
GDM now requires `NVreg_PreserveVideoMemoryAllocations` kernel parameter which breaks sleep in nvidia and hybrid mode, as well as rtd3 in hybrid mode, so EnvyControl disables it, if you need a Wayland session follow the instructions below:

  ```bash
  sudo systemctl enable nvidia-{suspend,resume,hibernate}
  sudo ln -s /dev/null /etc/udev/rules.d/61-gdm.rules
  ```

- **Display runs at 1 FPS when lid is closed on Hybrid Mode**</br>
This is a known bug with the proprietary Nvidia Linux drivers and Xorg PRIME implementation. There is no magic fix from EnvyControl's side. You can try disabling DRI3 by adding `LIBGL_DRI3_DISABLE=true` to your `/etc/environment`, but mileage may vary.

- **How do I completely uninstall and revert to defaults?**</br>
EnvyControl features a built-in total reset command. This safely removes all generated Udev, Modprobe, and Xorg configurations and rebuilds your initramfs to factory defaults.
  ```bash
  sudo envycontrol --reset
  ```

---

## 🤝 Contributing & License

EnvyControl is free and open-source software. Continuing the legacy of the original project, this rewrite is released under the [MIT License](LICENSE).

Found a bug? Have an idea? Pull Requests and Issues are highly welcome! Let's keep the Linux Optimus ecosystem thriving.
