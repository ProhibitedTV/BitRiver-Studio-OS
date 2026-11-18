#!/usr/bin/env bash
set -euo pipefail

# Evidence-only first-stage UEFI boot attempt for an ALREADY BUILT ISO.
# This is NOT a test of installation or installed-OS reboot.
# No host block devices, host shared folders, remote ports, or network access.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"
out="$repo_root/artifacts/iso-candidate"
mkdir -p "$repo_root/artifacts/iso-evidence"

mapfile -t iso_files < <(find "$out" -type f -name '*.iso')
[[ "${#iso_files[@]}" -eq 1 ]] || { echo "expected exactly one candidate ISO" >&2; exit 1; }
iso="${iso_files[0]}"
test -s "$iso"

code="/usr/share/OVMF/OVMF_CODE_4M.fd"
vars="/usr/share/OVMF/OVMF_VARS_4M.fd"
for name in qemu-system-x86_64 qemu-img socat; do
  command -v "$name" >/dev/null || { echo "$name unavailable" >&2; exit 1; }
done
[[ -s "$code" && -s "$vars" ]] || { echo "OVMF 4M firmware unavailable" >&2; exit 1; }

tmp="$(mktemp -d /tmp/bitriver-iso-uefi.XXXXXXXX)"
cleanup() {
  if [[ -e "$tmp/qemu.pid" ]]; then
    pid="$(cat "$tmp/qemu.pid" || true)"
    if [[ "$pid" =~ ^[0-9]+$ ]]; then
      kill "$pid" 2>/dev/null || true
    fi
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

# This 32 GiB disk is ALWAYS created inside a unique private /tmp directory;
# it cannot point at an existing disk. It is intentionally never installed
# to, attached to a host block device, or published.
qemu-img create -f qcow2 "$tmp/disposable-disk.qcow2" 32G >/dev/null
cp "$vars" "$tmp/OVMF_VARS.fd"
qemu-system-x86_64 \
  -machine q35,accel=kvm:tcg -cpu max -smp 2 -m 4096 \
  -drive "if=pflash,format=raw,readonly=on,file=$code" \
  -drive "if=pflash,format=raw,file=$tmp/OVMF_VARS.fd" \
  -drive "file=$iso,media=cdrom,readonly=on,format=raw" \
  -drive "file=$tmp/disposable-disk.qcow2,if=virtio,format=qcow2" \
  -boot order=d -vga std -nic none -display none \
  -vnc 127.0.0.1:7 \
  -monitor "unix:$tmp/monitor.sock,server=on,wait=off" \
  -serial "file:$tmp/serial.log" \
  -pidfile "$tmp/qemu.pid" -daemonize

# Firmware/GRUB/Anaconda take time to initialize on a CI VM. Capture the
# output for MANUAL review; screenshot existence is not an install pass.
sleep 70
test -s "$tmp/qemu.pid"
kill -0 "$(cat "$tmp/qemu.pid")"
for attempt in 1 2 3; do
  printf 'screendump %s\n' "$tmp/installer-boot.ppm" | \
    timeout 8 socat - UNIX-CONNECT:"$tmp/monitor.sock" > "$tmp/monitor.log" 2>&1 || true
  [[ -s "$tmp/installer-boot.ppm" ]] && break
  sleep 2
done
test -s "$tmp/installer-boot.ppm" || {
  echo "UEFI QEMU screen capture failed" >&2; exit 1;
}
head -c 2 "$tmp/installer-boot.ppm" | grep -q 'P6' || {
  echo "QEMU framebuffer is not a PPM image" >&2; exit 1;
}

cp "$tmp/installer-boot.ppm" "$repo_root/artifacts/iso-evidence/uefi-boot-capture.ppm"
cp "$tmp/serial.log" "$repo_root/artifacts/iso-evidence/uefi-serial.log"
cat > "$repo_root/artifacts/iso-evidence/uefi-boot-probe.json" <<'JSON'
{
  "schemaVersion": 1,
  "uefiVMLaunched": true,
  "framebufferCaptured": true,
  "networkConnected": false,
  "diskInstallationVerified": false,
  "rebootVerified": false,
  "releaseReady": false,
  "note": "Visual boot attempt evidence only. Manually inspect image for Anaconda; no disk installation was attempted."
}
JSON

echo "UEFI VM remained alive and framebuffer evidence was captured; Anaconda and installation are NOT verified."
