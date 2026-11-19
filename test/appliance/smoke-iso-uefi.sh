#!/usr/bin/env bash
set -euo pipefail

# Isolated first-stage UEFI boot gate for an ALREADY BUILT ISO.
# Requires proof the live installer reaches Anaconda's pre-service, not just
# GRUB or a framebuffer. Does NOT certify the Anaconda UI, install or reboot.
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

# This 64 GiB sparse disk is ALWAYS created inside a unique private /tmp directory;
# it cannot point at an existing disk. It is intentionally never installed
# to, attached to a host block device, or published.
qemu-img create -f qcow2 "$tmp/disposable-disk.qcow2" 64G >/dev/null
cp "$vars" "$tmp/OVMF_VARS.fd"
qemu-system-x86_64 \
  -machine q35,accel=kvm:tcg -cpu max -smp 4 -m 8192 \
  -drive "if=pflash,format=raw,readonly=on,file=$code" \
  -drive "if=pflash,format=raw,file=$tmp/OVMF_VARS.fd" \
  -drive "file=$iso,media=cdrom,readonly=on,format=raw" \
  -drive "file=$tmp/disposable-disk.qcow2,if=virtio,format=qcow2" \
  -boot order=d -vga std -nic none -display none \
  -vnc 127.0.0.1:7 \
  -monitor "unix:$tmp/monitor.sock,server=on,wait=off" \
  -serial "file:$tmp/serial.log" \
  -chardev "file,id=anacondalog,path=$tmp/anaconda-virtio.log" \
  -device virtio-serial-pci \
  -device "virtserialport,chardev=anacondalog,name=org.fedoraproject.anaconda.log.0" \
  -pidfile "$tmp/qemu.pid" -daemonize

# Capture three stages of UEFI progress, not just "QEMU is running".
# On GH-hosted runners TCG may boot very slowly without /dev/kvm.
capture_screen() {
  local target="$1" attempt
  for attempt in 1 2 3; do
    printf 'screendump %s\n' "$target" | \
      timeout 8 socat - UNIX-CONNECT:"$tmp/monitor.sock" > "$tmp/monitor.log" 2>&1 || true
    [[ -s "$target" ]] && break
    sleep 2
  done
  test -s "$target" || { echo "UEFI framebuffer capture failed" >&2; return 1; }
  [[ "$(head -c 2 "$target")" == "P6" ]] || {
    echo "UEFI framebuffer is not a PPM image" >&2; return 1;
  }
}

sleep 75
test -s "$tmp/qemu.pid"
kill -0 "$(cat "$tmp/qemu.pid")"
capture_screen "$tmp/early.ppm"
sleep 140
kill -0 "$(cat "$tmp/qemu.pid")"
capture_screen "$tmp/mid.ppm"
# Under TCG (no KVM), startup may be slow; the third capture at ~615s
# observes startup beyond the first systemd and Plymouth service events.
# This is still NOT an installer UI / installation / reboot proof.
sleep 400
kill -0 "$(cat "$tmp/qemu.pid")"
capture_screen "$tmp/later.ppm"

cp "$tmp/early.ppm" "$repo_root/artifacts/iso-evidence/uefi-boot-capture-early.ppm"
cp "$tmp/mid.ppm" "$repo_root/artifacts/iso-evidence/uefi-boot-capture-mid.ppm"
cp "$tmp/later.ppm" "$repo_root/artifacts/iso-evidence/uefi-boot-capture.ppm"
cp "$tmp/serial.log" "$repo_root/artifacts/iso-evidence/uefi-serial.log"
# Anaconda's documented, isolated virtio log port is independent of
# tty0/ttyS0 console precedence and never requires network access.
test -e "$tmp/anaconda-virtio.log" || : > "$tmp/anaconda-virtio.log"
cp "$tmp/anaconda-virtio.log" "$repo_root/artifacts/iso-evidence/anaconda-virtiolog.log"

# Fail if QEMU never reached even Anaconda's startup prerequisite. This
# prevents structurally valid but unbootable ISOs from passing the ISO job.
# This marker DOES NOT demonstrate an interactive Anaconda UI.
python3 test/appliance/analyze-iso-boot.py \
  "$tmp/early.ppm" "$tmp/mid.ppm" "$tmp/later.ppm" "$tmp/serial.log" \
  "$repo_root/artifacts/iso-evidence/uefi-boot-probe.json" \
  "$tmp/anaconda-virtio.log"
echo "PASS: UEFI reached systemd and logged Anaconda pre-service START; installation NOT validated."
