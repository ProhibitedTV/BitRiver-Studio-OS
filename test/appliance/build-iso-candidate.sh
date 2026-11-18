#!/usr/bin/env bash
set -euo pipefail

# Experimental offline generic-iso proof of concept. Produces only local
# test output + non-secret evidence. NO public release and NO disk install.
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
out="$root/artifacts/iso-candidate"
mkdir -p "$out"
test -z "$(find "$out" -mindepth 1 -maxdepth 1 -print -quit)" || {
  echo "Refusing to reuse nonempty ISO candidate output" >&2; exit 1;
}
payload=localhost/bitriver-studio-os:ci
installer=localhost/bitriver-installer:ci
fedora="$(jq -er '.builderImages.fedoraBootc | select(test("^quay[.]io/fedora/fedora-bootc@sha256:[a-f0-9]{64}$"))' deploy/image/releases.lock.json)"
builder="$(jq -er '.toolImages.imageBuilderCli | select(test("^ghcr[.]io/osbuild/image-builder-cli@sha256:[a-f0-9]{64}$"))' deploy/image/releases.lock.json)"
test "$fedora" = "$(sed -n 's/^ARG FEDORA_BOOTC_IMAGE=//p' deploy/iso/Containerfile)"

# Load the regular BitRiver payload into rootful Podman. Do not reuse the
# disposable CI QCOW2: it has a broad sudo-enabled SSH test account.
docker image inspect bitriver/studio-os:ci >/dev/null
docker tag bitriver/studio-os:ci "$payload"
docker save -o "$out/payload-oci.tar" "$payload"
sudo podman load -i "$out/payload-oci.tar"
rm -f "$out/payload-oci.tar"
sudo podman image exists "$payload"

# Build an independent Anaconda boot environment from the same pinned Fedora
# base. This IS NOT the appliance OS; the payload is embedded by Image Builder.
sudo podman build --platform linux/amd64 -f deploy/iso/Containerfile \
  --build-arg "FEDORA_BOOTC_IMAGE=$fedora" -t "$installer" .
sudo podman image exists "$installer"

# Generic ISO uses the new CLI (not the deprecated anaconda-iso type).
# The bootc payload must be present in local container storage and embedded.
sudo podman run --rm --privileged \
  -v "$out:/output" \
  -v /var/lib/containers/storage:/var/lib/containers/storage \
  "$builder" \
  --bootc-ref "$installer" \
  --bootc-installer-payload-ref "$payload" \
  --bootc-default-fs ext4 \
  build --with-manifest generic-iso
sudo chown -R "$(id -u):$(id -g)" "$out"

mapfile -t iso_files < <(find "$out" -type f -name '*.iso')
test "${#iso_files[@]}" -eq 1 || { echo "Expected exactly one generated ISO" >&2; exit 1; }
iso="${iso_files[0]}"
test -s "$iso"

# Capture diagnostics before assertions; GitHub preserves metadata only.
# Image Builder generic-iso layouts differ from older live installer layouts.
xorriso -indev "$iso" -report_el_torito plain > "$out/boot-record.txt" 2>&1
# xorriso -find defaults to the "echo" action. "-print" is not a
# supported xorriso find option (unlike GNU find).
xorriso -indev "$iso" -find / -type f > "$out/iso-file-list.txt" 2>&1
(cd "$(dirname "$iso")" && sha256sum "$(basename "$iso")") > "$out/ISO-SHA256SUMS"

# Use a fixture-tested parser for the actual Fedora generic-iso layout.
# xorriso quotes every file path (e.g. '/LiveOS/squashfs.img'); earlier
# regexes falsely rejected images which were built successfully.
python3 test/appliance/check-iso-layout.py "$out/boot-record.txt" "$out/iso-file-list.txt"

# Retain the REAL generated boot parameters for a subsequent isolated
# Anaconda install/reboot test; don't invent a kernel command line.
xorriso -indev "$iso" -osirrox on -extract /boot/grub2/grub.cfg \
  "$out/bios-grub.cfg" > "$out/bios-grub-extract.log" 2>&1
xorriso -indev "$iso" -osirrox on -extract /EFI/BOOT/grub.cfg \
  "$out/uefi-grub.cfg" > "$out/uefi-grub-extract.log" 2>&1
test -s "$out/bios-grub.cfg"
test -s "$out/uefi-grub.cfg"

jq -n --arg builder "$builder" --arg source "${GITHUB_SHA:-unknown}" \
  '{schemaVersion:1, releaseReady:false, artifactType:"experimental offline installer ISO",
    sourceRevision:$source, imageBuilderRef:$builder,
    bootSmokePassed:false, installationSmokePassed:false,
    updateOrigin:"CI-only local placeholder; not configured for updates",
    safety:"NO PUBLIC RELEASE: interactive installer is unverified; never run against important disks"}' \
  > "$out/iso-provenance.json"
# We intentionally do not archive, upload or copy the ISO into any public
# release assets. Only hashes/provenance/layout evidence can leave this job.
echo "Experimental generic-iso created and structure checked; installation NOT validated."
