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
grep -Fq -- '-no-reboot' test/appliance/smoke-iso-uefi.sh
grep -Fq -- '-vga virtio' test/appliance/smoke-iso-uefi.sh
grep -Fq 'ISO smoke failed; captured available guest logs' test/appliance/smoke-iso-uefi.sh
grep -Fq 'bootPrerequisitePassed == true' .github/workflows/appliance-iso.yml
grep -Fq 'analyze-iso-boot.py' test/appliance/smoke-iso-uefi.sh
grep -Fq 'bootPrerequisitePassed == true' test/appliance/package-iso-qa.sh
grep -Fq 'finalFramebufferNonUniform == true' .github/workflows/appliance-iso.yml
grep -Fq 'finalFramebufferNonUniform == true' test/appliance/package-iso-qa.sh
grep -Fq 'requiredSmokePassed == true' .github/workflows/appliance-iso.yml
grep -Fq 'requiredSmokePassed == true' test/appliance/package-iso-qa.sh
grep -Fq 'installerInitiatedReboot == false' .github/workflows/appliance-iso.yml
grep -Fq 'firefoxStartedUnprivileged == true' .github/workflows/appliance-iso.yml
grep -Fq 'firefoxStartedUnprivileged == true' test/appliance/package-iso-qa.sh
grep -Fq 'browserProbeTelemetrySeen == true' .github/workflows/appliance-iso.yml
grep -Fq 'browserProbeTelemetrySeen == true' test/appliance/package-iso-qa.sh
grep -Fq 'browserProbeOrderingCycleDetected == false' .github/workflows/appliance-iso.yml
grep -Fq 'browserProbeOrderingCycleDetected == false' test/appliance/package-iso-qa.sh
grep -Fq 'installerInitiatedReboot == false' test/appliance/package-iso-qa.sh
grep -Fq 'event_name == ' .github/workflows/appliance-iso.yml
python3 test/appliance/test-iso-layout.py -v
grep -Fq 'python3 test/appliance/check-iso-layout.py' test/appliance/build-iso-candidate.sh
test -s deploy/iso/Containerfile
test -s deploy/iso/99-bitriver-browser.conf
grep -Fxq 'webui_web_engine = firefox' deploy/iso/99-bitriver-browser.conf
grep -Fq 'COPY deploy/iso/99-bitriver-browser.conf /etc/anaconda/conf.d/99-bitriver-browser.conf' deploy/iso/Containerfile
grep -Fq 'useradd --system --create-home' deploy/iso/Containerfile
grep -Fq 'test "$browser_user" = bitriver-installer' deploy/iso/Containerfile
grep -Fq 'anaconda anaconda-webui firefox anaconda-install-img-deps' deploy/iso/Containerfile
grep -Fq 'rpm -q anaconda-webui firefox' deploy/iso/Containerfile
grep -Fq 'test -x /usr/libexec/anaconda/webui-desktop' deploy/iso/Containerfile
test -s deploy/iso/iso.yaml
test -s deploy/iso/interactive-defaults.ks
test -s deploy/iso/serial-boot-diagnostics.conf
test -s deploy/iso/iso-browser-probe.sh
test -s deploy/iso/iso-browser-probe.service
bash -n deploy/iso/iso-browser-probe.sh
grep -Fq 'BITRIVER_ISO_PROBE sample=' deploy/iso/iso-browser-probe.sh
grep -Fxq 'After=basic.target' deploy/iso/iso-browser-probe.service
! grep -Fxq 'After=anaconda.service' deploy/iso/iso-browser-probe.service
grep -Fq 'ConditionKernelCommandLine=inst.graphical' deploy/iso/iso-browser-probe.service
grep -Fq 'COPY deploy/iso/iso-browser-probe.sh' deploy/iso/Containerfile
grep -Fq 'anaconda.target.wants/bitriver-iso-browser-probe.service' deploy/iso/Containerfile
grep -Fxq 'ForwardToConsole=yes' deploy/iso/serial-boot-diagnostics.conf
grep -Fxq 'TTYPath=/dev/ttyS0' deploy/iso/serial-boot-diagnostics.conf
grep -Fq 'COPY deploy/iso/serial-boot-diagnostics.conf' deploy/iso/Containerfile
grep -Fq 'org.fedoraproject.anaconda.log.0' test/appliance/smoke-iso-uefi.sh
test -s .github/workflows/appliance-iso.yml

# Keep the payload and ISO installer built from the reviewed Fedora digest.
fedora="$(jq -er '.builderImages.fedoraBootc' deploy/image/releases.lock.json)"
grep -Fxq "ARG FEDORA_BOOTC_IMAGE=$fedora" deploy/iso/Containerfile
grep -Fq 'FROM ${FEDORA_BOOTC_IMAGE}' deploy/iso/Containerfile
grep -Fxq 'bootc --source-imgref containers-storage:localhost/bitriver-studio-os:ci --target-imgref localhost/bitriver-studio-os:ci' deploy/iso/interactive-defaults.ks
grep -Fq -- '--bootc-installer-payload-ref "$payload"' test/appliance/build-iso-candidate.sh
grep -Fq 'generic-iso' test/appliance/build-iso-candidate.sh
grep -Fq 'check-iso-grub.py' test/appliance/build-iso-candidate.sh
grep -Fq 'inst.graphical rd.plymouth=0 plymouth.enable=0 loglevel=7 systemd.show_status=yes rd.systemd.show_status=yes console=ttyS0,115200n8 console=tty0' deploy/iso/iso.yaml
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
 deploy/iso/iso-browser-probe.service
grep -Fq 'ConditionKernelCommandLine=inst.graphical' deploy/iso/iso-browser-probe.service
grep -Fq 'COPY deploy/iso/iso-browser-probe.sh' deploy/iso/Containerfile
grep -Fq 'anaconda.target.wants/bitriver-iso-browser-probe.service' deploy/iso/Containerfile
grep -Fxq 'ForwardToConsole=yes' deploy/iso/serial-boot-diagnostics.conf
grep -Fxq 'TTYPath=/dev/ttyS0' deploy/iso/serial-boot-diagnostics.conf
grep -Fq 'COPY deploy/iso/serial-boot-diagnostics.conf' deploy/iso/Containerfile
grep -Fq 'org.fedoraproject.anaconda.log.0' test/appliance/smoke-iso-uefi.sh
test -s .github/workflows/appliance-iso.yml

# Keep the payload and ISO installer built from the reviewed Fedora digest.
fedora="$(jq -er '.builderImages.fedoraBootc' deploy/image/releases.lock.json)"
grep -Fxq "ARG FEDORA_BOOTC_IMAGE=$fedora" deploy/iso/Containerfile
grep -Fq 'FROM ${FEDORA_BOOTC_IMAGE}' deploy/iso/Containerfile
grep -Fxq 'bootc --source-imgref containers-storage:localhost/bitriver-studio-os:ci --target-imgref localhost/bitriver-studio-os:ci' deploy/iso/interactive-defaults.ks
grep -Fq -- '--bootc-installer-payload-ref "$payload"' test/appliance/build-iso-candidate.sh
grep -Fq 'generic-iso' test/appliance/build-iso-candidate.sh
grep -Fq 'check-iso-grub.py' test/appliance/build-iso-candidate.sh
grep -Fq 'inst.graphical rd.plymouth=0 plymouth.enable=0 loglevel=7 systemd.show_status=yes rd.systemd.show_status=yes console=ttyS0,115200n8 console=tty0' deploy/iso/iso.yaml
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
