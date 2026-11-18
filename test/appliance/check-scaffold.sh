#!/usr/bin/env bash
set -euo pipefail

# A fast scaffold check, not a substitute for booting a real bootc image.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

test -f apps/control-api/cmd/bitriver-hw-report/main.go
test -f apps/control-api/cmd/bitriver-operator-key/main.go
grep -Fq 'COPY --from=api-build /out/bitriver-operator-key /usr/local/bin/bitriver-operator-key' deploy/image/Containerfile
test -f deploy/image/rootfs-overlay/usr/local/libexec/bitriver/firstboot.sh
grep -Fq 'COPY --from=api-build /out/bitriver-hw-report /usr/local/bin/bitriver-hw-report' deploy/image/Containerfile
grep -Fq '/usr/local/bin/bitriver-hw-report --output' deploy/image/rootfs-overlay/usr/local/libexec/bitriver/firstboot.sh

echo 'Checking security configuration...'
test -f deploy/image/rootfs-overlay/etc/ssh/sshd_config.d/50-bitriver.conf
grep -Fq 'ln -s /var/lib/bitriver/data /data' deploy/image/Containerfile
grep -Fq 'systemd-networkd.service sshd.service' deploy/image/Containerfile
grep -Fq 'BITRIVER_HTTP_ADDR=127.0.0.1:8080' deploy/image/rootfs-overlay/etc/bitriver/bitriver.env
grep -Fq -- '-operator-credentials-path' deploy/systemd/bitriver-control-api.service
grep -Fq -- '-adapter-mode appliance' deploy/systemd/bitriver-control-api.service
grep -Fq 'controlKeySha256' deploy/image/rootfs-overlay/usr/local/libexec/bitriver/firstboot.sh
grep -Fq 'http://127.0.0.1:8080/' deploy/image/rootfs-overlay/etc/sway/config.d/bitriver-kiosk.conf

echo 'Checking immutable OCI builder image references...'
bash test/appliance/check-image-lock.sh
bash test/appliance/check-npm-lock.sh

echo 'Checking appliance script syntax...'
bash -n test/appliance/check-release-assets.sh
bash -n test/appliance/check-image-lock.sh
bash -n test/appliance/check-npm-lock.sh
bash -n test/appliance/build-qcow2.sh
bash -n test/appliance/build-virtualbox-preview.sh
grep -Fq 'preview ALL=(root) NOPASSWD: /usr/bin/cat /var/lib/bitriver/bootstrap/one-time-admin-password.txt' test/appliance/build-virtualbox-preview.sh
grep -Fq 'if: ${{ github.event_name !=' .github/workflows/virtualbox-preview.yml
bash -n test/appliance/validate-oci-sbom.sh
bash -n test/appliance/smoke-uefi-qcow2.sh
while IFS= read -r -d '' script; do
  bash -n "${script}"
done < <(find deploy/image/rootfs-overlay/usr/local/libexec/bitriver -name '*.sh' -print0)

echo 'Checking JSON templates...'
jq -e . deploy/image/rootfs-overlay/usr/local/share/bitriver/templates/initial-config.template.json >/dev/null

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT
echo 'Building and running first-boot hardware probe...'
(
  cd apps/control-api
  go build -o "${tmp_dir}/bitriver-hw-report" ./cmd/bitriver-hw-report
)
"${tmp_dir}/bitriver-hw-report" --output "${tmp_dir}/capabilities.json"

jq -e '
  (.detectedAt | type == "string") and
  (.architecture | type == "string") and
  (.cpuModel | type == "string") and
  (.logicalCores | type == "number") and
  (.memoryMiB | type == "number") and
  (.displayAttached | type == "boolean") and
  (.bootFirmware == "uefi" or .bootFirmware == "bios" or .bootFirmware == "unknown") and
  (.gpus | type == "array") and
  (.networkAdapters | type == "array") and
  (.compatibilityNotes | type == "array") and
  (.videoDevices | type == "array") and
  (.audioCaptureDevices | type == "array") and
  (.networkInterfaces | type == "array") and
  (.capabilities.kioskEligible | type == "boolean") and
  (.capabilities.obsEligible | type == "boolean") and
  (.capabilities.recommendedForMvp | type == "boolean")
' "${tmp_dir}/capabilities.json" >/dev/null

echo 'Building offline operator recovery command...'
(
  cd apps/control-api
  go build -o "${tmp_dir}/bitriver-operator-key" ./cmd/bitriver-operator-key
)
echo 'Appliance scaffold and hardware report checks passed.'
