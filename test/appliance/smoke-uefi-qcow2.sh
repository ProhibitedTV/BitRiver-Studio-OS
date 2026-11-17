#!/usr/bin/env bash
set -euo pipefail

# Boot a disposable QCOW2 using real OVMF UEFI firmware. Exercise firstboot,
# authenticated operator bootstrap files, control API, and reboot persistence.
# This VM uses a CI-only test user provisioned at disk-conversion time.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
output_dir="${repo_root}/artifacts/vm"
qcow2="$(cat "${output_dir}/qcow2-path.txt")"
test -f "${qcow2}"
test -f "${output_dir}/id_ed25519"
mkdir -p "${output_dir}/diagnostics"

if [[ -e /usr/share/OVMF/OVMF_CODE_4M.fd && -e /usr/share/OVMF/OVMF_VARS_4M.fd ]]; then
  firmware_code=/usr/share/OVMF/OVMF_CODE_4M.fd
  firmware_vars=/usr/share/OVMF/OVMF_VARS_4M.fd
elif [[ -e /usr/share/OVMF/OVMF_CODE.fd && -e /usr/share/OVMF/OVMF_VARS.fd ]]; then
  firmware_code=/usr/share/OVMF/OVMF_CODE.fd
  firmware_vars=/usr/share/OVMF/OVMF_VARS.fd
else
  echo "OVMF UEFI firmware not installed" >&2
  exit 1
fi
cp "${firmware_vars}" "${output_dir}/OVMF_VARS_test.fd"

qemu-system-x86_64 \
  -machine q35,accel=tcg -cpu max -smp 2 -m 4096 \
  -drive if=pflash,format=raw,unit=0,readonly=on,file="${firmware_code}" \
  -drive if=pflash,format=raw,unit=1,file="${output_dir}/OVMF_VARS_test.fd" \
  -drive if=virtio,format=qcow2,file="${qcow2}" \
  -netdev user,id=net0,hostfwd=tcp:127.0.0.1:22222-:22 \
  -device virtio-net-pci,netdev=net0 \
  -display none -monitor none -serial file:"${output_dir}/diagnostics/uefi-console.log" &
qemu_pid=$!
cleanup() {
  kill "${qemu_pid}" 2>/dev/null || true
  wait "${qemu_pid}" 2>/dev/null || true
}
trap cleanup EXIT

ssh_opts=(-F /dev/null -p 22222 -i "${output_dir}/id_ed25519"
  -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=4
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null
  -o LogLevel=ERROR)
guest() {
  ssh "${ssh_opts[@]}" bitriver-vmtest@127.0.0.1 "$@"
}
wait_for_guest() {
  local expected="${1:-}"
  local boot_id
  for ((attempt=1; attempt<=160; attempt++)); do
    if ! kill -0 "${qemu_pid}" 2>/dev/null; then
      echo "QEMU exited before the guest became accessible" >&2
      return 1
    fi
    boot_id="$(guest cat /proc/sys/kernel/random/boot_id 2>/dev/null || true)"
    if [[ -n "${boot_id}" && "${boot_id}" != "${expected}" ]]; then
      printf '%s\n' "${boot_id}"
      return 0
    fi
    sleep 3
  done
  echo "VM did not accept SSH or finish reboot within the timeout" >&2
  return 1
}

diagnostics() {
  # Never collect bootstrap recovery notices or credential contents.
  guest 'sudo -n /bin/bash -se' > "${output_dir}/diagnostics/unit-status.txt" 2>&1 <<'GUEST' || true
systemctl --no-pager --plain --failed
systemctl --no-pager --plain status bitriver-firstboot.service bitriver.target bitriver-control-api.service mediamtx.service bitriver-update-agent.service
journalctl --no-pager --output=short -u bitriver-firstboot.service -u bitriver-control-api.service -u mediamtx.service -n 100
GUEST
}
wait_for_appliance() {
  local phase="$1"
  for ((attempt=1; attempt<=100; attempt++)); do
    if guest 'sudo -n systemctl is-active --quiet bitriver.target && sudo -n systemctl is-active --quiet bitriver-firstboot.service && sudo -n systemctl is-active --quiet bitriver-control-api.service && sudo -n systemctl is-active --quiet mediamtx.service && sudo -n systemctl is-active --quiet bitriver-update-agent.service && curl -fsS http://127.0.0.1:8080/healthz >/dev/null'; then
      echo "BitRiver appliance is ready (${phase})"
      return 0
    fi
    # A failed required unit cannot make progress without intervention.
    if guest 'sudo -n systemctl is-failed --quiet bitriver-firstboot.service' 2>/dev/null; then
      echo "Firstboot systemd unit failed during ${phase}" >&2
      diagnostics
      return 1
    fi
    sleep 3
  done
  echo "Timed out waiting for BitRiver target/services (${phase})" >&2
  diagnostics
  return 1
}

echo "Waiting for first UEFI boot to enable SSH..."
first_boot_id="$(wait_for_guest)"
echo "VM first boot ID: ${first_boot_id}"
wait_for_appliance "first boot"

if ! guest 'sudo -n /bin/bash -se' <<'GUEST'
set -euo pipefail
systemctl is-active --quiet bitriver.target
systemctl is-active --quiet bitriver-firstboot.service
systemctl is-active --quiet bitriver-control-api.service
systemctl is-active --quiet mediamtx.service
systemctl is-active --quiet bitriver-update-agent.service
curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null
jq -e '.firstBootComplete == true and .credentialsReady == true and
  (.state == "ready" or .state == "degraded")' \
  /var/lib/bitriver/bootstrap/setup-status.json >/dev/null
jq -e '.capabilities | type == "object"' /var/lib/bitriver/hardware/capabilities.json >/dev/null
jq -e '.controlKeySha256 | type == "string" and length == 64' \
  /var/lib/bitriver/bootstrap/admin-credentials.json >/dev/null
test "$(stat -c '%a' /var/lib/bitriver/bootstrap/admin-credentials.json)" = 600
test "$(stat -c '%U' /var/lib/bitriver/bootstrap/admin-credentials.json)" = bitriver
test -L /data
test -d /data/recordings
test "$(stat -c '%U' /data/recordings)" = studio
echo "Firstboot, systemd target, credentials, health API and persistent /data passed."
GUEST
then
  diagnostics
  exit 1
fi

baseline="$(guest 'sudo -n sha256sum /var/lib/bitriver/bootstrap/device-identity.json /var/lib/bitriver/bootstrap/admin-credentials.json')"
printf '%s\n' "${baseline}" > "${output_dir}/diagnostics/before-reboot-sha256.txt"

echo "Rebooting UEFI VM to check persistent identity and operator key..."
guest 'sudo -n systemctl reboot' >/dev/null 2>&1 || true
second_boot_id="$(wait_for_guest "${first_boot_id}")"
echo "VM reboot ID: ${second_boot_id}"
wait_for_appliance "second boot"

if ! guest 'sudo -n /bin/bash -se' <<'GUEST'
set -euo pipefail
systemctl is-active --quiet bitriver.target
systemctl is-active --quiet bitriver-control-api.service
curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null
test -d /data/recordings
GUEST
then
  diagnostics
  exit 1
fi
after="$(guest 'sudo -n sha256sum /var/lib/bitriver/bootstrap/device-identity.json /var/lib/bitriver/bootstrap/admin-credentials.json')"
printf '%s\n' "${after}" > "${output_dir}/diagnostics/after-reboot-sha256.txt"
if [[ "${baseline}" != "${after}" ]]; then
  echo "Identity or operator credentials changed unexpectedly after reboot" >&2
  exit 1
fi

echo "PASS: UEFI QEMU first boot, required services, API and reboot persistence."
