#!/usr/bin/env bash
set -euo pipefail

# Manual opt-in only. Never published as a GitHub Release or PR artifact.
if [[ "${GITHUB_EVENT_NAME:-}" != "workflow_dispatch" ||
      "${BITRIVER_QA_OPT_IN:-}" != "true" ]]; then
  echo "Refusing ISO export: manual dispatch and explicit QA opt-in required" >&2
  exit 1
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
input="$root/artifacts/iso-candidate"
output="$root/artifacts/iso-qa-download"
test ! -e "$output" || { echo "QA output already exists" >&2; exit 1; }
mapfile -t isos < <(find "$input" -type f -name '*.iso' -print)
[[ "${#isos[@]}" -eq 1 ]] || { echo "expected exactly one ISO" >&2; exit 1; }
iso="${isos[0]}"
test -s "$iso"
test -s "$input/boot-record.txt"
test -s "$input/iso-file-list.txt"
test -s "$input/iso-provenance.json"
test -s "$root/artifacts/iso-evidence/uefi-boot-probe.json"
jq -e '.bootPrerequisitePassed == true and
       .finalFramebufferNonUniform == true and .finalFramebufferUniform == false and
       .requiredSmokePassed == true and
       .anacondaWebuiDesktopLaunched == true and
       .firefoxBrowserSelected == true and
       .firefoxStartedUnprivileged == true and
       .installerInitiatedReboot == false and
       .browserSandboxRefusalSeen == false and
       .releaseReady == false and
       .anacondaUserInterfaceVerified == false and
       .diskInstallationVerified == false and .rebootVerified == false' \
  "$root/artifacts/iso-evidence/uefi-boot-probe.json" >/dev/null
jq -e '.releaseReady == false and
       .bootSmokePassed == false and
       .installationSmokePassed == false and
       .artifactType == "experimental offline installer ISO"' \
  "$input/iso-provenance.json" >/dev/null
python3 test/appliance/check-iso-layout.py "$input/boot-record.txt" "$input/iso-file-list.txt"

mkdir -p "$output/chunks"
split -b 900M -d -a 3 -- "$iso" "$output/chunks/bitriver-studio-os-qa.iso.part"
test "$(find "$output/chunks" -name '*.part*' -type f | wc -l)" -ge 2
sha="$(sha256sum "$iso" | awk '{print $1}')"
printf '%s  bitriver-studio-os-qa.iso\n' "$sha" > "$output/ISO-SHA256SUMS"
(cd "$output" && sha256sum chunks/* > CHUNK-SHA256SUMS)
cp "$input/iso-provenance.json" "$output/iso-provenance.json"
cp "$root/artifacts/iso-evidence/uefi-boot-probe.json" "$output/uefi-boot-probe.json"
cp "$input/boot-record.txt" "$output/boot-record.txt"
cp "$input/iso-file-list.txt" "$output/iso-file-list.txt"
cp "$root/test/appliance/reassemble-iso-qa.ps1" "$output/reassemble-iso-qa.ps1"
cat > "$output/READ-ME-FIRST.txt" <<'TEXT'
BITRIVER STUDIO OS — EXPERIMENTAL ISO FOR ISOLATED VM QA ONLY

NOT A RELEASE. NOT VALIDATED TO INSTALL, BOOT, UPDATE OR ROLLBACK.
DO NOT WRITE TO USB OR INSTALL ON PHYSICAL COMPUTERS.
USE A BLANK DISPOSABLE VIRTUAL DISK WITH NO HOST STORAGE PASS-THROUGH.

This manual workflow artifact is NOT a signed production build or GitHub
Release. Files are ordered binary chunks in ./chunks. Reconstruct and
verify SHA256 against the manifest before attaching to a virtual machine.
The uefi-boot-probe.json requires a prerequisite boot event and a
non-uniform final VM framebuffer. These are not a verified Anaconda UI,
installation to disk, reboot, updates or rollback.

Windows PowerShell in this folder:
  .\reassemble-iso-qa.ps1

Linux/macOS in this folder:
  cat chunks/bitriver-studio-os-qa.iso.part* > bitriver-studio-os-qa.iso
  sha256sum -c ISO-SHA256SUMS

Follow docs/ISO_VM_INSTALL_QA.md in the repository in a disposable VM.
The installer currently uses a CI-only update image reference; do not
consider the installed OS to have safe unattended updates or rollback.
TEXT
test -s "$output/reassemble-iso-qa.ps1"
echo "Staged isolated-VM QA bundle, NOT a release: SHA256=$sha"
