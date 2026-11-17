#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

lock=package-lock.json
jq -e '
  .name == "bitriver-studio-os" and
  .lockfileVersion == 3 and
  (.packages[""].name == "bitriver-studio-os") and
  (.packages["apps/control-ui"].name == "@bitriver/control-ui") and
  (.packages["packages/shared"].name == "@bitriver/shared") and
  (.packages["node_modules/@bitriver/control-ui"].link == true) and
  (.packages["node_modules/@bitriver/shared"].link == true) and
  (all(.packages | to_entries[];
    if (.key | startswith("node_modules/")) and ((.value.link // false) | not)
    then
      (.value.integrity | test("^sha512-[A-Za-z0-9+/]+=*$")) and
      (.value.resolved | startswith("https://registry.npmjs.org/"))
    else true end))
' "${lock}" >/dev/null

grep -Fq "COPY package.json package-lock.json" deploy/image/Containerfile
grep -Fq "RUN npm ci --ignore-scripts --no-audit --no-fund" deploy/image/Containerfile
grep -Fq "npm ci --ignore-scripts --no-audit --no-fund" .github/workflows/ci.yml

if grep -Eq "RUN npm install|^[[:space:]]*run: npm install" \
  deploy/image/Containerfile .github/workflows/ci.yml; then
  echo "Mutable npm install detected in a production or CI build" >&2
  exit 1
fi

echo "npm lockfile: workspaces, trusted registry URLs and SHA-512 integrities verified."
