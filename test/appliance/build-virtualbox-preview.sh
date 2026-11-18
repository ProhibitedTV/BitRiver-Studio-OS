#!/usr/bin/env bash
set -euo pipefail

# A separate, deliberately non-production VirtualBox preview disk.
# NEVER distribute artifacts/vm/disk.qcow2: it contains disposable CI SSH
# credentials and a passwordless administrator.
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${root}"
build_dir="${root}/artifacts/virtualbox/build"
publish_dir="${root}/artifacts/virtualbox/publish"
mkdir -p "${build_dir}" "${publish_dir}"
test -z "$(find "${publish_dir}" -mindepth 1 -maxdepth 1 -print -quit)" || {
  echo "Refusing to reuse a nonempty VirtualBox publishing directory" >&2
  exit 1
}
name=BitRiver-Studio-OS-VirtualBox-Preview-1
image_ref=localhost/bitriver-studio-os:ci
builder="$(jq -er '.toolImages.imageBuilderCli |
  select(test("^ghcr[.]io/osbuild/image-builder-cli@sha256:[a-f0-9]{64}$"))' \
  deploy/image/releases.lock.json)"

# The only published account is a console-only preview user. It is NOT in
# wheel and may ONLY read the one-time local operator key with sudo.
# Password SSH access is disabled by the underlying appliance image.
jq -n '{
  customizations: {
    user: [{name: "preview", password: "bitriver-preview"}],
    files: [{
      path: "/etc/sudoers.d/90-bitriver-vbox-preview",
      mode: "0440",
      data: "preview ALL=(root) NOPASSWD: /usr/bin/cat /var/lib/bitriver/bootstrap/one-time-admin-password.txt\n"
    }]
  }
}' > "${build_dir}/preview-blueprint.json"
jq -e '
  (.customizations.user | length == 1) and
  .customizations.user[0].name == "preview" and
  (.customizations.user[0] | has("key") | not) and
  (.customizations.user[0] | has("groups") | not) and
  (.customizations.files | length == 1) and
  (.customizations.files[0].data |
    contains("NOPASSWD: /usr/bin/cat /var/lib/bitriver/bootstrap/one-time-admin-password.txt"))
' "${build_dir}/preview-blueprint.json" >/dev/null

# The CI smoke job ordinarily loaded this image into Podman already.
if ! sudo podman image exists "${image_ref}"; then
  docker image inspect bitriver/studio-os:ci >/dev/null
  docker tag bitriver/studio-os:ci "${image_ref}"
  docker save -o "${build_dir}/oci.tar" "${image_ref}"
  sudo podman load -i "${build_dir}/oci.tar"
  rm -f "${build_dir}/oci.tar"
fi
sudo podman image exists "${image_ref}"

# Image Builder supports qcow2; convert that proven bootc disk to VirtualBox's
# native VDI format without copying in CI VM user or key customizations.
sudo podman run --rm --privileged \
  -v "${build_dir}:/output" \
  -v "${build_dir}/preview-blueprint.json:/config.json:ro" \
  -v /var/lib/containers/storage:/var/lib/containers/storage \
  "${builder}" \
  --bootc-ref "${image_ref}" \
  --bootc-default-fs ext4 \
  build --blueprint /config.json --with-manifest qcow2
sudo chown -R "$(id -u):$(id -g)" "${build_dir}"
qcow="$(find "${build_dir}" -type f -name '*.qcow2' -print -quit)"
test -s "${qcow}" || { echo "Image Builder did not generate a preview disk" >&2; exit 1; }
vdi="${build_dir}/${name}.vdi"
qemu-img convert -p -f qcow2 -O vdi "${qcow}" "${vdi}"
qemu-img info --output=json "${vdi}" | jq -e '
  .format == "vdi" and ."virtual-size" >= 1073741824
' >/dev/null
test -s "${vdi}"

# A reproducible manifest and the complete archive are the only user assets.
cp docs/VIRTUALBOX_PREVIEW.md "${publish_dir}/README-VIRTUALBOX.txt"
(cd "${build_dir}" && sha256sum "${name}.vdi") > \
  "${publish_dir}/VDI-SHA256SUMS"
jq -n \
  --arg type "VirtualBox native VDI" \
  --arg builder "${builder}" \
  --arg source "${GITHUB_SHA:-unknown}" \
  --arg warnings "Experimental, known console preview password, NAT-only, no unattended production use" \
  '{schemaVersion:1, releaseReady:false, platform:"linux/amd64",
    diskType:$type, imageBuilderRef:$builder, sourceRevision:$source,
    securityNotice:$warnings}' > "${publish_dir}/preview-provenance.json"

archive="${publish_dir}/${name}.zip"
(cd "${build_dir}" && zip -q -9 "${archive}" "${name}.vdi")
test -s "${archive}"
(cd "${publish_dir}" && sha256sum "${name}.zip") > \
  "${publish_dir}/ZIP-SHA256SUMS"
# Each GitHub Release asset must fit below 2 GiB. Split only if needed.
max_asset_size=1700000000
if (( $(stat -c %s "${archive}") > max_asset_size )); then
  split -b "${max_asset_size}" -d -a 2 \
    "${archive}" "${archive}.part-"
  rm -f "${archive}"
  echo "Preview ZIP split into numbered parts; use README reassembly steps."
fi
(cd "${publish_dir}" && sha256sum \
  "${name}.zip"* README-VIRTUALBOX.txt preview-provenance.json) > \
  "${publish_dir}/ASSETS-SHA256SUMS"

# Detect accidental inclusion of any CI-only keys, disk or VM blueprint.
if find "${publish_dir}" -type f \( -name '*.qcow2' -o -name '*id_ed25519*' \
    -o -name 'preview-blueprint.json' -o -name '*.vdi' \) | grep -q .; then
  echo "A forbidden CI secret or raw test disk entered publish assets" >&2
  exit 1
fi
echo "Built isolated VirtualBox test image; only publish/ contains release assets."
find "${publish_dir}" -maxdepth 1 -type f -printf '%f %s bytes\n'
