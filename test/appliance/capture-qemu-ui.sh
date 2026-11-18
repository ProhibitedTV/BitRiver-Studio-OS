#!/usr/bin/env bash
set -euo pipefail
# Boots only the disposable CI QCOW2 (never published); signs in over a
# loopback SSH tunnel to the real control API and captures browser screenshots.
# The original CI VM has an ephemeral SSH account and a one-time operator key.
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
umask 077
vm="$root/artifacts/vm"
disk="$(cat "$vm/qcow2-path.txt")"
test -s "$disk"
test -s "$vm/id_ed25519"
out="$root/docs/screenshots/real"
mkdir -p "$out" "$vm/diagnostics"
temp="$(mktemp -d)"
qemu_pid=""
tunnel_pid=""
cleanup() {
  if [[ -n "$tunnel_pid" ]]; then kill "$tunnel_pid" 2>/dev/null || true; wait "$tunnel_pid" 2>/dev/null || true; fi
  if [[ -n "$qemu_pid" ]]; then kill "$qemu_pid" 2>/dev/null || true; wait "$qemu_pid" 2>/dev/null || true; fi
  rm -rf "$temp"
}
trap cleanup EXIT

if [[ -e /usr/share/OVMF/OVMF_CODE_4M.fd && -e /usr/share/OVMF/OVMF_VARS_4M.fd ]]; then
  firmware_code=/usr/share/OVMF/OVMF_CODE_4M.fd
  firmware_vars=/usr/share/OVMF/OVMF_VARS_4M.fd
elif [[ -e /usr/share/OVMF/OVMF_CODE.fd && -e /usr/share/OVMF/OVMF_VARS.fd ]]; then
  firmware_code=/usr/share/OVMF/OVMF_CODE.fd
  firmware_vars=/usr/share/OVMF/OVMF_VARS.fd
else
  echo 'OVMF firmware missing' >&2; exit 1
fi
cp "$firmware_vars" "$temp/OVMF_VARS.fd"
qemu-system-x86_64 \
  -machine q35,accel=tcg -cpu max -smp 2 -m 4096 \
  -drive if=pflash,format=raw,unit=0,readonly=on,file="$firmware_code" \
  -drive if=pflash,format=raw,unit=1,file="$temp/OVMF_VARS.fd" \
  -drive if=virtio,format=qcow2,file="$disk" \
  -netdev user,id=net0,hostfwd=tcp:127.0.0.1:22222-:22 \
  -device virtio-net-pci,netdev=net0 \
  -display none -monitor none -serial file:"$vm/diagnostics/screenshot-console.log" &
qemu_pid=$!

ssh_opts=(-F /dev/null -p 22222 -i "$vm/id_ed25519"
  -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=4
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR)
guest() { ssh "${ssh_opts[@]}" bitriver-vmtest@127.0.0.1 "$@"; }
ready=0
for ((i=1; i<=160; i++)); do
  kill -0 "$qemu_pid" 2>/dev/null || { echo 'VM exited before services started' >&2; exit 1; }
  if guest 'sudo -n systemctl is-active --quiet bitriver.target && sudo -n systemctl is-active --quiet bitriver-control-api.service && curl -fsS http://127.0.0.1:8080/healthz >/dev/null' 2>/dev/null; then
    ready=1; break
  fi
  sleep 3
done
test "$ready" -eq 1 || { echo 'VM never reached healthy BitRiver control API' >&2; exit 1; }

# The key file exists only in a private temporary directory; never upload it.
guest 'sudo -n sed -n "s/^Operator control key: //p" /var/lib/bitriver/bootstrap/one-time-admin-password.txt' > "$temp/operator-key"
chmod 0600 "$temp/operator-key"
test -s "$temp/operator-key"

ssh "${ssh_opts[@]}" -o ExitOnForwardFailure=yes \
  -L 127.0.0.1:18080:127.0.0.1:8080 -N bitriver-vmtest@127.0.0.1 &
tunnel_pid=$!
tunnel_ready=0
for ((i=1; i<=30; i++)); do
  if curl -fsS http://127.0.0.1:18080/healthz >/dev/null 2>&1; then
    tunnel_ready=1; break
  fi
  kill -0 "$tunnel_pid" 2>/dev/null || { echo 'SSH tunnel failed' >&2; exit 1; }
  sleep 1
done
test "$tunnel_ready" -eq 1 || { echo 'Cannot reach VM localhost through tunnel' >&2; exit 1; }

# Playwright-core is test-only and uses the preinstalled runner Chromium.
# It does not modify the production npm lockfile or base OS.
npm install --prefix "$temp/capture" --no-save --ignore-scripts --no-audit --no-fund playwright-core@1.57.0 >/dev/null
chrome="$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)"
test -n "$chrome" || { echo 'No Chrome/Chromium executable on capture runner' >&2; exit 1; }
BITRIVER_PLAYWRIGHT_MODULE="$temp/capture/node_modules/playwright-core" \
BITRIVER_CAPTURE_OUTPUT="$out" \
BITRIVER_CAPTURE_URL=http://127.0.0.1:18080/ \
BITRIVER_CAPTURE_KEY_FILE="$temp/operator-key" \
BITRIVER_CHROME="$chrome" \
node test/appliance/capture-qemu-ui.mjs

for png in operator-login.png appliance-dashboard.png appliance-sources.png; do
  test -s "$out/$png"
  file "$out/$png" | grep -q 'PNG image data'
done
cat > "$out/PROVENANCE.md" <<EOF
# Verified BitRiver appliance UI captures

These three PNGs were captured from the **actual control UI hosted by a
disposable Fedora bootc BitRiver Studio OS guest** running under
QEMU q35 with OVMF UEFI. A loopback-only SSH tunnel reached
the guest's real local API and a headless Chrome browser captured its
operator login, authenticated dashboard and source list.

- Source revision: ${GITHUB_SHA:-local-developer-build}
- Build run: https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/${GITHUB_RUN_ID:-unknown}
- Test-only guest: no physical capture card, real streaming or graphical
  Wayland/OBS session was certified
- No operator keys, test SSH keys or CI QCOW2 images are committed

These are **real running-appliance web UI** screenshots, not screenshots
of the guest's display or a physical/VirtualBox graphics certification.
EOF
echo 'Real VM control UI captures passed; no credentials or VM disk exported.'
