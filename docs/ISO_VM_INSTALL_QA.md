# BitRiver Studio OS ISO — disposable VM acceptance test

**Pre-release QA only.** This is a reproducible checklist for judging whether
an experimental ISO is usable. Do not install the candidate on a real PC or a
disk containing important data. **No clean installation has yet passed this
checklist.** The ISO pipeline normally retains only build evidence. It also offers
a **manual, short-lived unvalidated QA artifact** behind a separate explicit
opt-in on the `appliance-iso-experimental` workflow; this is not a public
installer download or production release.

## Automated evidence (what CI does and does not prove)

The experimental ISO workflow now requires QEMU/OVMF to boot into systemd
and log Anaconda's `anaconda-pre.service`. It samples the framebuffer
at roughly 75, 215 and 395 seconds to accommodate slower emulated CPUs.
The QA bundle includes `uefi-boot-probe.json`: check
`bootPrerequisitePassed`, `anacondaPreServiceStarted`,
`anacondaPreServiceFinished` and `finalFramebufferUniform` separately.
The most recent verified screenshot (~395 seconds into the software-emulated
QEMU boot) was a **uniform blue frame**, not a proven Anaconda screen.
**Do not** equate the positive prerequisite with the Anaconda UI
being visible. The `anacondaUserInterfaceVerified`,
`diskInstallationVerified`, `rebootVerified` and `releaseReady` flags
must stay false until independently verified and documented.
A successful CI job is necessary, **not sufficient**, for a stable release.

## Test environment

- Use **VirtualBox** or QEMU/OVMF on an isolated computer.
- New VM: x86_64, **UEFI enabled**, at least 4 vCPUs, 8 GiB RAM, VMSVGA,
  dynamic blank virtual disk **64 GiB or larger**. This disk must be separate
  from the host; do not pass through any host drive.
- Disconnect the VM network until after first boot and credential review.
- Attach exactly one unverified ISO as virtual optical media.
- Record the test date, ISO SHA256, source commit, VM firmware mode, host CPU,
  installation target disk name/size and installed OS version.

## Install and reboot (human verification required)

1. Boot ISO. Photograph/screenshot the initial **BitRiver Studio OS** boot
   menu and select the explicitly interactive graphical or text installer.
2. Confirm Anaconda launches and offers a **manual** target-disk selection.
   If the default graphical installer remains blank, power off and retry
   using the second GRUB entry **Troubleshoot installation (text, basic display)**.
   It uses `inst.text nomodeset rd.plymouth=0 plymouth.enable=0`. Record
   whether the serial console displays Anaconda text mode; **do not**
   assume that reaching the boot menu means text mode works.
   **Fail immediately** if it starts repartitioning without the operator
   choosing a disk and confirming all changes.
3. Confirm the only install target is the disposable 64+ GiB VM disk.
   Configure the appropriate installer user/credentials and manually
   acknowledge any formatting step. Do not enter personal credentials.
4. Check that the installer can find the **offline embedded BitRiver bootc**
   payload without contacting an external registry.
5. Complete installation and record any warnings, exit code or error text.
   A successful Anaconda window alone is NOT a pass.
6. Power down, **detach the ISO**, and boot from the installed virtual disk.
   Record whether the firmware successfully loads the new OS.
7. Confirm the first-boot service completes. Validate identity, operator
   authentication, mount layout, MediaMTX, OBS startup posture, hardware
   inventory, and local API availability. Headless OBS may legitimately
   report unavailable; it must not crash the operator UI.
8. Reboot **again** without ISO and confirm the appliance ID, configuration,
   operator key and recording storage persist.

## In-guest checks (run as a local administrator)

```bash
cat /etc/os-release
uname -m
lsblk -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINTS
findmnt /
findmnt /var
systemctl is-active bitriver.target
systemctl --no-pager --full status bitriver-firstboot.service
systemctl --no-pager --full status bitriver-control-api.service
curl --fail --silent --show-error http://127.0.0.1:8080/healthz
curl --fail --silent --show-error -I http://127.0.0.1:8080/
sudo stat -c '%a %U:%G %n' /var/lib/bitriver /var/lib/bitriver/data
```

An API `/healthz` response is only service liveness—not OBS, streaming,
recording or full-media-pipeline certification. The operator UI may require
the locally generated password/key from first boot. Do **not** post secrets,
private URLs, SSH keys or session cookies in issue logs or screenshots.

## Pass conditions

| Gate | Required evidence |
| --- | --- |
| ISO integrity | SHA256 verified against the build's signed/checksummed artifact manifest |
| Firmware boot | Native VM UEFI boot menu, without QEMU direct-kernel shortcuts |
| Installer safety | Manual disk selection and confirmation; no unattended wipe |
| Offline payload | Installation succeeds while the VM has no external network |
| Disk installation | BitRiver installed on blank target; no dependency on the ISO afterward |
| First boot | Local API, systemd target and firstboot records report expected health |
| Persistence | Second reboot retains system ID, operator credentials, settings and recordings |
| Security | No public admin endpoint, test SSH credential, default root key or disabled auth |
| Recovery | Interrupted install leaves host disks untouched; upgrade rollback separately demonstrated |
| Evidence | Screenshots, redacted logs, checksums and test-machine specifications attached |

Do not label the ISO a **functional release** until all installation/boot
gates are checked and reproducible. A **non-release QA ISO** may be used for
this testing with explicit warnings. Physical PC validation, publishing
rights, media-stack end-to-end testing, update origin and rollback are
separate public-release blockers.

## Report a failed test

Use [GitHub issues](https://github.com/ProhibitedTV/BitRiver-Studio-OS/issues)
and include: the SHA256, which numbered step failed, the relevant
`journalctl -b --no-pager -u bitriver-firstboot.service -u bitriver-control-api.service`
output with credentials redacted, and a screenshot of the failure if safe.
Do not attach whole virtual disks or authenticated browser profiles.

## Obtaining a test ISO (manual opt-in only)

From the repository's **Actions → appliance-iso-experimental → Run workflow**,
select branch `build/experimental-offline-installer-iso` and explicitly check
`export_qa_iso`. The workflow must successfully generate and structurally
validate one ISO before packaging any download. GitHub Actions then retains
the `bitriver-experimental-iso-ISOLATED-VM-QA-NOT-RELEASE` artifact for
seven days. Pull requests and ordinary pushes **never** upload an ISO.

Download/extract the full artifact, including its numbered `chunks/`
directory, `ISO-SHA256SUMS` and `READ-ME-FIRST.txt`.

On **Windows PowerShell**, from the extracted folder run
`./reassemble-iso-qa.ps1`. It refuses to overwrite an existing ISO and
checks SHA256 before reporting success. On Linux/macOS, concatenate chunks
in filename order and verify the `ISO-SHA256SUMS` manifest.

The result is `bitriver-studio-os-qa.iso`, an **experimental test input**
for an **isolated virtual machine with an empty disposable disk**. The
download does not demonstrate successful installation, rollback, an
update origin, hardware support or security. **Never flash this QA build
onto a real PC.**
