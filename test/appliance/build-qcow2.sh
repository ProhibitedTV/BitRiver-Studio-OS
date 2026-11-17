#!/usr/bin/env bash
set -euo pipefail

# Convert the tested local Docker OCI image to a UEFI-capable QCOW2 disk.
# Only the disposable CI VM gets this randomly generated SSH test user.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
output_dir="${repo_root}/artifacts/vm"
mkdir -p "${output_dir}"
test -f "${output_dir}/id_ed25519.pub" || {
  echo "Missing disposable VM SSH public key" >&2
  exit 1
}
test ! -e "${output_dir}/disk.qcow2" || {
  echo "Refusing to overwrite an existing VM disk image" >&2
  exit 1
}

# The bootstrap account and passwordless sudo exist ONLY in the generated
# disposable VM disk, not in the bootc container, source repo, or releases.
jq -n --arg key "$(cat "${output_dir}/id_ed25519.pub")" '{
  customizations: {
    user: [{name: "bitriver-vmtest", groups: ["wheel"], key: $key}],
    files: [{
      path: "/etc/sudoers.d/90-bitriver-vmtest",
      mode: "0440",
      data: "bitriver-vmtest ALL=(root) NOPASSWD: ALL\n"
    }],
    kernel: {append: "console=ttyS0"}
  }
}' > "${output_dir}/vm-blueprint.json"

docker image inspect bitriver/studio-os:ci >/dev/null
docker tag bitriver/studio-os:ci localhost/bitriver-studio-os:ci
docker save -o "${output_dir}/bootc-local-image.tar" localhost/bitriver-studio-os:ci
sudo podman load -i "${output_dir}/bootc-local-image.tar"
rm -f "${output_dir}/bootc-local-image.tar"
sudo podman image exists localhost/bitriver-studio-os:ci

# Pin the build tooling as tightly as the Fedora, Go and Node image stages.
# "latest" would silently change the disk layout or generated SBOM.
image_builder_ref="$(jq -er '.toolImages.imageBuilderCli |
  select(test("^ghcr[.]io/osbuild/image-builder-cli@sha256:[a-f0-9]{64}$"))' \
  deploy/image/releases.lock.json)"
echo "Pinned Image Builder CLI: ${image_builder_ref}"

# image-builder-cli succeeds bootc-image-builder. This uses local container
# storage and does not upload the image to any public registry.
sudo podman run --rm --privileged \
  -v "${output_dir}:/output" \
  -v "${output_dir}/vm-blueprint.json:/config.json:ro" \
  -v /var/lib/containers/storage:/var/lib/containers/storage \
  "${image_builder_ref}" \
  --bootc-ref localhost/bitriver-studio-os:ci \
  --bootc-default-fs ext4 \
  build --blueprint /config.json --with-manifest --with-sbom qcow2

sudo chown -R "$(id -u):$(id -g)" "${output_dir}"

# --with-sbom generates SPDX documents for the image OS and buildroot.
# Save inventory evidence only; NEVER upload the CI disk or VM blueprint.
sbom_dir="${output_dir}/sbom"
mkdir -p "${sbom_dir}"
image_os_sbom=""
count=0
while IFS= read -r -d '' sbom; do
  jq -e '
    .spdxVersion == "SPDX-2.3" and
    (.packages | type == "array") and
    (.packages | length > 0)
  ' "${sbom}" >/dev/null || {
    echo "Invalid or empty SPDX document: ${sbom}" >&2
    exit 1
  }
  cp -- "${sbom}" "${sbom_dir}/$(basename "${sbom}")"
  count=$((count+1))
  if [[ "${sbom}" == *.image-os.spdx.json ]]; then
    image_os_sbom="${sbom}"
  fi
done < <(find "${output_dir}" -path "${sbom_dir}" -prune -o \
  -type f -name '*.spdx.json' -print0)
if [[ "${count}" -lt 2 || -z "${image_os_sbom}" ]]; then
  echo "Expected image OS and buildroot SPDX documents; found ${count}" >&2
  find "${output_dir}" -type f -name '*spdx*' -print >&2
  exit 1
fi

jq -n --arg toolRef "${image_builder_ref}" --argjson count "${count}" \
  --slurpfile sbom "${image_os_sbom}" '{
    schemaVersion: 1,
    releaseReady: false,
    imageBuilderRef: $toolRef,
    spdxDocuments: $count,
    imageOsPackageCount: ($sbom[0].packages | length),
    imageOsPackagesWithDeclaredLicense: ([
      $sbom[0].packages[] |
      select((.licenseDeclared // "NOASSERTION") != "NOASSERTION")
    ] | length),
    inventoryScope: "image-builder bootc disk conversion; excludes separately bundled binaries and npm packages"
  }' > "${sbom_dir}/inventory-summary.json"
(cd "${sbom_dir}" && sha256sum -- *.spdx.json > SHA256SUMS)
echo "Verified ${count} SPDX documents; saving non-release inventory evidence."

qcow2="$(find "${output_dir}" -type f -name '*.qcow2' -print -quit)"
test -n "${qcow2}"
qemu-img info --output=json "${qcow2}" | jq -e '.format == "qcow2" and (."virtual-size" >= 1073741824)' >/dev/null
printf '%s\n' "${qcow2}" > "${output_dir}/qcow2-path.txt"
sha256sum "${qcow2}" > "${output_dir}/qcow2.sha256"
cat "${output_dir}/qcow2.sha256"
echo "Built QCOW2 for disposable UEFI VM validation only; no public release image."
