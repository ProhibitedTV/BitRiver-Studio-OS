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

# Validate actual ISO filesystem entries and UEFI boot descriptors.
xorriso -indev "$iso" -report_el_torito plain > "$out/boot-record.txt" 2>&1
grep -Eiq 'UEFI|EFI' "$out/boot-record.txt"
for entry in /LiveOS/squashfs.img /images/pxeboot/vmlinuz /images/pxeboot/initrd.img; do
  xorriso -indev "$iso" -find "$entry" -print 2>/dev/null | grep -Fq "$entry"
done

(cd "$(dirname "$iso")" && sha256sum "$(basename "$iso")") > "$out/ISO-SHA256SUMS"
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
