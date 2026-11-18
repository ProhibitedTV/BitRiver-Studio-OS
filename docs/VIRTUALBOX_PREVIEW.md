# BitRiver Studio OS — VirtualBox Technical Preview 1

**EXPERIMENTAL TEST DISK — NOT FOR PRODUCTION, REAL BROADCASTS, OR INTERNET EXPOSURE.**

This is a dedicated **x86_64 VirtualBox VDI**, built from the same Fedora bootc
image validated by CI. It is **not** the CI test disk, a supported installer,
a signed release, or a hardened remote-access appliance.

## Download and verify

Download the files from the **VirtualBox Technical Preview 1** entry under the
GitHub repository's Releases page. Get the ZIP (or every numbered ZIP part),
`ZIP-SHA256SUMS`, and `README-VIRTUALBOX.txt`.

If you have a normal `BitRiver-Studio-OS-VirtualBox-Preview-1.zip`, skip
reassembly. If the ZIP is split into `.part-00`, `.part-01`, etc., put all
parts in the same folder and run the following in **PowerShell** from there:

```powershell
$zip = "BitRiver-Studio-OS-VirtualBox-Preview-1.zip"
$parts = @(Get-ChildItem -File "$zip.part-*" | Sort-Object Name)
if (!(Test-Path $zip) -and $parts.Count -gt 0) {
    $output = [System.IO.File]::Create((Join-Path (Get-Location) $zip))
    try {
        foreach ($part in $parts) {
            $inputFile = [System.IO.File]::OpenRead($part.FullName)
            try { $inputFile.CopyTo($output) }
            finally { $inputFile.Dispose() }
        }
    } finally { $output.Dispose() }
}
(Get-FileHash $zip -Algorithm SHA256).Hash
```

Compare the resulting hash to the hash recorded in `ZIP-SHA256SUMS`.
The separate `ASSETS-SHA256SUMS` lists all uploaded ZIP or split part hashes.
For extra verification, compare the VDI SHA-256 against `VDI-SHA256SUMS`
after extracting the ZIP. Do not proceed if checksums differ.

Extract the ZIP using Windows Explorer. The resulting file is
`BitRiver-Studio-OS-VirtualBox-Preview-1.vdi`.

## Create a VirtualBox VM

1. Install VirtualBox 7.x on an x86_64 host and create a **New** VM using
   **Linux / Other Linux (64-bit)**. Do not select unattended OS installation.
2. Give it **4 virtual CPUs and 8 GiB of memory**, if your computer permits.
3. Under **System > Motherboard**, enable **EFI** firmware.
4. Under **Storage**, attach the extracted VDI to a **SATA controller** as the
   existing virtual hard disk. Do not attach a fresh blank disk instead.
5. Under **Display**, use **VMSVGA** with **128 MB** video memory. Try enabling
   3D acceleration only if needed; Wayland/OBS graphics are not certified.
6. Under **Network**, use **NAT only**. Do **not** use bridged networking, port
   forwarding or public inbound exposure for this technical preview.
7. Start the VM. Fedora and BitRiver first boot may require several minutes.

Do **not** attach the VDI to a physical disk. This is not a flashable
USB installer. It is a VirtualBox testing disk.

## Console login and first-run checks

The downloadable preview disk **only** adds this restricted *local console*
account:

- Username: `preview`
- Password: `bitriver-preview`

**Change that password immediately** after first login by running `passwd`.
This account is not a member of `wheel`. No CI SSH key or broad
passwordless-sudo account is included in this distributed disk. The OS
disallows SSH password authentication, so this documented password cannot
be used to log in over SSH.

At a terminal or Linux text console in the guest, sign in as `preview` and run:

```bash
systemctl is-active bitriver.target
systemctl is-active bitriver-control-api.service
curl -fsS http://127.0.0.1:8080/healthz
sudo /usr/bin/cat /var/lib/bitriver/bootstrap/one-time-admin-password.txt
```

The final command is the **only privileged operation granted to the preview
account**. It prints the generated local operator key, which you can use to
sign in through the guest's BitRiver UI at `http://127.0.0.1:8080/`.
Treat the key as a secret; do not paste it into issue reports.

If the graphical kiosk is unavailable, use a text TTY (VirtualBox menu:
**Input > Keyboard**, send Ctrl+Alt+F2) to inspect systemd status and the
health endpoint. The QEMU tests verify headless services, **not** working
OBS graphics or VirtualBox VMSVGA acceleration. A blank or nonfunctional
kiosk must be treated as a known test limitation, not proof the OS passed a
graphical hardware test.

## Known limitations and safety

- **Not signed**, audited, rollback-tested, or approved for a real stream.
- Known initial local console password, meant for isolated testing **only**.
- No hardware capture cards, audio devices, OBS/Wayland display, remote admin,
  update safety, or stream durability is certified yet.
- The application API binds to loopback inside the VM. NAT does not mean
  the host can directly browse the guest's `127.0.0.1`; use the VM console.
- Use a **throwaway VM**. Do not reuse its operator key, passwords or VM disk
  for an internet-connected appliance or production broadcast.
- The base OS is not guaranteed byte-for-byte reproducible from mutable Fedora
  RPM repositories; check the downloadable build provenance and SBOM separately.

For a bug report, include your VirtualBox version, EFI/CPU/RAM/display settings,
what appeared on the VM console, and the **status text only** from the above
commands. Redact any credentials, session tokens, RTMP keys, or private files.

Repository: https://github.com/ProhibitedTV/BitRiver-Studio-OS
Release blockers: https://github.com/ProhibitedTV/BitRiver-Studio-OS/issues/4
