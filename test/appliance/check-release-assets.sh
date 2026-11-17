#!/usr/bin/env bash
set -euo pipefail

# Check exact locked inputs, not caller-controlled archive overrides.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
lock="${repo_root}/deploy/image/releases.lock.json"
temp_dir="$(mktemp -d)"
trap 'rm -rf "${temp_dir}"' EXIT

jq -e '
  .schemaVersion == 1 and
  .targetPlatform == "linux/amd64" and
  .fedoraBootc == "quay.io/fedora/fedora-bootc:44" and
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
grep -Eq '(^|^\./)mediamtx

echo "Locked media artifacts match release SHA256 digests and expected archive layouts."
 "${temp_dir}/mediamtx-members.txt"
grep -Eq '(^|^\./)owncast

echo "Locked media artifacts match release SHA256 digests and expected archive layouts."
 "${temp_dir}/owncast-members.txt"

echo "Locked media artifacts match release SHA256 digests and expected archive layouts."
