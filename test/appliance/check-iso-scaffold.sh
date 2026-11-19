#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
bash -n test/appliance/build-iso-candidate.sh
bash -n test/appliance/smoke-iso-uefi.sh
python3 test/appliance/test-iso-boot-evidence.py -v
python3 test/appliance/test-iso-grub.py -v
test -s test/appliance/analyze-iso-boot.py
bash -n test/appliance/package-iso-qa.sh
test -s test/appliance/reassemble-iso-qa.ps1
grep -Fq 'GITHUB_EVENT_NAME' test/appliance/package-iso-qa.sh
grep -Fq 'BITRIVER_QA_OPT_IN' test/appliance/package-iso-qa.sh
grep -Fq 'inputs.export_qa_iso == true' .github/workflows/appliance-iso.yml
grep -Fq 'run: bash test/appliance/smoke-iso-uefi.sh' .github/workflows/appliance-iso.yml
grep -Fq 'bootPrerequisitePassed == true' .github/workflows/appliance-iso.yml
grep -Fq 'analyze-iso-boot.py' test/appliance/smoke-iso-uefi.sh
grep -Fq 'bootPrerequisitePassed == true' test/appliance/package-iso-qa.sh
grep -Fq 'event_name == ' .github/workflows/appliance-iso.yml
python3 test/appliance/test-iso-layout.py -v
grep -Fq 'python3 test/appliance/check-iso-layout.py' test/appliance/build-iso-candidate.sh
test -s deploy/iso/Containerfile
test -s deploy/iso/iso.yaml
test -s deploy/iso/interactive-defaults.ks
test -s .github/workflows/appliance-iso.yml

# Keep the payload and ISO installer built from the reviewed Fedora digest.
fedora="$(jq -er '.builderImages.fedoraBootc' deploy/image/releases.lock.json)"
grep -Fxq "ARG FEDORA_BOOTC_IMAGE=$fedora" deploy/iso/Containerfile
grep -Fq 'FROM ${FEDORA_BOOTC_IMAGE}' deploy/iso/Containerfile
grep -Fxq 'bootc --source-imgref containers-storage:localhost/bitriver-studio-os:ci --target-imgref localhost/bitriver-studio-os:ci' deploy/iso/interactive-defaults.ks
grep -Fq -- '--bootc-installer-payload-ref "$payload"' test/appliance/build-iso-candidate.sh
grep -Fq 'generic-iso' test/appliance/build-iso-candidate.sh
grep -Fq 'check-iso-grub.py' test/appliance/build-iso-candidate.sh
grep -Fq 'inst.graphical console=ttyS0,115200n8 console=tty0' deploy/iso/iso.yaml
grep -Fq 'inst.text nomodeset rd.plymouth=0 plymouth.enable=0' deploy/iso/iso.yaml

# Fail if unattended or destructive installer settings land on this branch.
if grep -Ein '(^|[[:space:]])(clearpart|autopart|zerombr|ignoredisk|part[[:space:]]|reqpart|rootpw|user[[:space:]]|reboot|shutdown)([[:space:]]|$)|inst[.](ks|cmdline|noninteractive)|--source-imgref[[:space:]]+registry:' \
    deploy/iso/interactive-defaults.ks deploy/iso/iso.yaml | grep -v '^.*:#'; then
  echo 'Unsafe or network-dependent installer default detected' >&2; exit 1
fi
# Never auto-upload an ISO from a PR or publish a release; a manual-only
# short-lived QA artifact is allowed, with a second package-script check.
! grep -Eq 'gh release|contents: write|upload-release|upload-artifact.*iso|publish-iso' .github/workflows/appliance-iso.yml
grep -Fq 'ISO-SHA256SUMS' test/appliance/build-iso-candidate.sh
grep -Fq 'bootSmokePassed:false' test/appliance/build-iso-candidate.sh
echo 'Experimental ISO scaffold guard passed.'
