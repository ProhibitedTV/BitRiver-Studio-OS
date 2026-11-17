#!/usr/bin/env bash
set -euo pipefail

# Validate exact upstream archives and layout, without accepting URL overrides.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
lock="${repo_root}/deploy/image/releases.lock.json"
temp_dir="$(mktemp -d)"
trap 'rm -rf "${temp_dir}"' EXIT

jq -e '
  .schemaVersion == 2 and
  .targetPlatform == "linux/amd64" and
  .fedoraBootc == "quay.io/fedora/fedora-bootc:44" and
  .builderImages.fedoraBootc == "quay.io/fedora/fedora-bootc@sha256:5f2947d685c24029b2eb4e1f0634dc4ac06116a88d9dd93da4b5571b2f0e28e1" and
  .builderImages.go == "docker.io/library/golang:1.22-bookworm@sha256:3d699e4d15d0f8f13c9195c0632a16702b8cbdece2955af1c23b37ae5d55a253" and
  .builderImages.node == "docker.io/library/node:22-bookworm@sha256:e13726ca604184d7a91fcacab1fc09eefd642dc28e79ac34e85540dd93b78cfb" and
  .mediamtx.version == "v1.21.4" and
  .mediamtx.archive == "mediamtx_v1.21.4_linux_amd64.tar.gz" and
  .mediamtx.url == "https://github.com/bluenviron/mediamtx/releases/download/v1.21.4/mediamtx_v1.21.4_linux_amd64.tar.gz" and
  (.mediamtx.sha256 | test("^[a-f0-9]{64}$")) and
  .owncast.version == "v0.3.0" and
  .owncast.archive == "owncast-0.3.0-linux-64bit.zip" and
  .owncast.url == "https://github.com/owncast/owncast/releases/download/v0.3.0/owncast-0.3.0-linux-64bit.zip" and
  (.owncast.sha256 | test("^[a-f0-9]{64}$")) and
  .mediamtx.license == "MIT" and .owncast.license == "MIT"
' "${lock}" >/dev/null

for component in mediamtx owncast; do
  url="$(jq -r ".${component}.url" "${lock}")"
  sha="$(jq -r ".${component}.sha256" "${lock}")"
  archive="$(jq -r ".${component}.archive" "${lock}")"
  printf 'Fetching locked %s release (%s)...\n' "${component}" "${archive}"
  curl -fL --retry 3 --proto '=https' --tlsv1.2 --output "${temp_dir}/${archive}" "${url}"
  printf '%s  %s\n' "${sha}" "${temp_dir}/${archive}" | sha256sum -c -
done

tar -tzf "${temp_dir}/mediamtx_v1.21.4_linux_amd64.tar.gz" > "${temp_dir}/mediamtx-members.txt"
unzip -Z1 "${temp_dir}/owncast-0.3.0-linux-64bit.zip" > "${temp_dir}/owncast-members.txt"
grep -Eq '^(\./)?mediamtx$' "${temp_dir}/mediamtx-members.txt"
grep -Eq '^(\./)?owncast$' "${temp_dir}/owncast-members.txt"

echo "Locked media artifacts match release SHA256 digests and expected archive layouts."
