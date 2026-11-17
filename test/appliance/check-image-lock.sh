#!/usr/bin/env bash
set -euo pipefail

# Refuse mutable or mismatched Dockerfile FROM references at review time.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
lock=deploy/image/releases.lock.json
file=deploy/image/Containerfile

jq -e '
  .schemaVersion == 2 and
  (.builderImages | all(.[]; test("@sha256:[a-f0-9]{64}$"))) and
  .builderImages.fedoraBootc == "quay.io/fedora/fedora-bootc@sha256:5f2947d685c24029b2eb4e1f0634dc4ac06116a88d9dd93da4b5571b2f0e28e1" and
  .builderImages.go == "docker.io/library/golang:1.22-bookworm@sha256:3d699e4d15d0f8f13c9195c0632a16702b8cbdece2955af1c23b37ae5d55a253" and
  .builderImages.node == "docker.io/library/node:22-bookworm@sha256:e13726ca604184d7a91fcacab1fc09eefd642dc28e79ac34e85540dd93b78cfb"
' "${lock}" >/dev/null

for entry in "FEDORA_BOOTC_IMAGE fedoraBootc" "GO_BUILDER_IMAGE go" "NODE_BUILDER_IMAGE node"; do
  read -r arg key <<<"${entry}"
  ref="$(jq -r --arg key "${key}" '.builderImages[$key]' "${lock}")"
  grep -Fxq "ARG ${arg}=${ref}" "${file}" || {
    echo "Unpinned or mismatched ${arg} in Containerfile" >&2
    exit 1
  }
  grep -Fxq "FROM ${${arg}}" "${file}" || {
    echo "Missing FROM reference ${arg} in Containerfile" >&2
    exit 1
  }
done
if grep -E '^FROM[[:space:]]+(docker\.io/|quay\.io/|golang:|node:)' "${file}"; then
  echo "Mutable or unlocked literal FROM reference in Containerfile" >&2
  exit 1
fi
echo "All OCI build stages are pinned to reviewed immutable digests."
