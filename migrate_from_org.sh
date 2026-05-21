#!/bin/sh
# Script di cư dữ liệu từ EnvyControl Python (V1) sang EnvyControl Go (V2)

OLD_CACHE="/var/cache/envycontrol/cache.json"
NEW_STATE_DIR="/etc/envycontrol"
NEW_STATE_FILE="$NEW_STATE_DIR/state.json"

# Yêu cầu quyền root
if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: Run this script with sudo."
    exit 1
fi

if [ ! -f "$OLD_CACHE" ]; then
    echo "INFO: No legacy cache found at $OLD_CACHE. Nothing to migrate."
    exit 0
fi

echo "INFO: Found legacy cache. Starting migration..."

# 1. Trích xuất PCI Bus ID từ file JSON cũ (Dùng cut/grep thay vì jq để thuần POSIX)
PCI_BUS=$(grep '"nvidia_gpu_pci_bus"' "$OLD_CACHE" | cut -d '"' -f 4)

if [ -z "$PCI_BUS" ]; then
    echo "ERROR: Could not parse PCI Bus from legacy cache."
    exit 1
fi

# 2. Quét iGPU Vendor
IGPU_VENDOR="unknown"
if lspci | grep -iE "VGA compatible controller|Display controller" | grep -iq "Intel"; then
    IGPU_VENDOR="intel"
elif lspci | grep -iE "VGA compatible controller|Display controller" | grep -iqE "AMD|ATI"; then
    IGPU_VENDOR="amd"
fi

# 3. Đoán Mode hiện tại
CURRENT_MODE="hybrid"
if [ -f "/etc/modprobe.d/blacklist-nvidia.conf" ] && { [ -f "/etc/udev/rules.d/50-remove-nvidia.rules" ] || [ -f "/lib/udev/rules.d/50-remove-nvidia.rules" ]; }; then
    CURRENT_MODE="integrated"
elif [ -f "/etc/X11/xorg.conf" ] && [ -f "/etc/modprobe.d/nvidia.conf" ]; then
    CURRENT_MODE="nvidia"
fi

# 4. Tạo thư mục và ghi file State mới
mkdir -p "$NEW_STATE_DIR"
chmod 755 "$NEW_STATE_DIR"

cat <<EOF > "$NEW_STATE_FILE"
{
    "current_mode": "$CURRENT_MODE",
    "igpu_vendor": "$IGPU_VENDOR",
    "nvidia_pci_bus": "$PCI_BUS"
}
EOF
chmod 644 "$NEW_STATE_FILE"

# 5. Dọn dẹp tàn dư cũ
rm -f "$OLD_CACHE"
rmdir /var/cache/envycontrol 2>/dev/null || true

echo "SUCCESS: Migration completed successfully!"
echo "New state saved to $NEW_STATE_FILE"
