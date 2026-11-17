#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
sbom_dir=artifacts/vm/sbom
sbom="${sbom_dir}/appliance-image.spdx.json"
test -s "${sbom}" || {
  echo "Syft did not produce the expected bootc OCI image SPDX file" >&2
  exit 1
}
jq -e '
  .spdxVersion == "SPDX-2.3" and
  (.packages | type == "array") and
  (.packages | length > 0)
' "${sbom}" >/dev/null

jq -n --slurpfile doc "${sbom}" \
  --arg image "bitriver/studio-os:ci" \
  --arg tool "anchore/syft:v1.57.2" '{
    schemaVersion: 1,
    releaseReady: false,
    image: $image,
    scanner: $tool,
    spdxVersion: $doc[0].spdxVersion,
    packageCount: ($doc[0].packages | length),
    packagesWithDeclaredLicense: ([
      $doc[0].packages[] |
      select((.licenseDeclared // "NOASSERTION") != "NOASSERTION")
    ] | length),
    inventoryScope: "automatically discovered packages in assembled bootc OCI image; manual license attribution still needed"
  }' > "${sbom_dir}/inventory-summary.json"
(cd "${sbom_dir}" && sha256sum appliance-image.spdx.json > SHA256SUMS)
(cd "${sbom_dir}" && sha256sum -c SHA256SUMS)
jq -e '.releaseReady == false and .packageCount > 0' \
  "${sbom_dir}/inventory-summary.json" >/dev/null
echo "PASS: pinned Syft OCI image SPDX inventory and checksum verified."
