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

# image-builder-cli succeeds bootc-image-builder. This uses local container
# storage and does not upload the image to any public registry.
sudo podman run --rm --privileged \
  -v "${output_dir}:/output" \
  -v "${output_dir}/vm-blueprint.json:/config.json:ro" \
  -v /var/lib/containers/storage:/var/lib/containers/storage \
  ghcr.io/osbuild/image-builder-cli:latest \
  --bootc-ref localhost/bitriver-studio-os:ci \
  --bootc-default-fs ext4 \
  build --blueprint /config.json --with-manifest --with-sbom qcow2

sudo chown -R "$(id -u):$(id -g)" "${output_dir}"
qcow2="$(find "${output_dir}" -type f -name '*.qcow2' -print -quit)"
test -n "${qcow2}"
qemu-img info --output=json "${qcow2}" | jq -e '.format == "qcow2" and (."virtual-size" >= 1073741824)' >/dev/null
printf '%s\n' "${qcow2}" > "${output_dir}/qcow2-path.txt"
sha256sum "${qcow2}" > "${output_dir}/qcow2.sha256"
cat "${output_dir}/qcow2.sha256"
echo "Built QCOW2 for disposable UEFI VM validation only; no public release image."
