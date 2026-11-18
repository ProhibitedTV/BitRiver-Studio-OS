# BitRiver Studio OS: normal PC hardware compatibility

**Goal:** A flexible open-source-first creator OS for ordinary x86_64 PCs,
gaming desktops, mini PCs, laptops and DIY media systems. Reusing a gaming
computer as a streaming rig should be a first-class use case, **not** a
requirement to buy a dedicated appliance.

**Status:** Hardware inventory and Fedora Mesa/firmware baseline implemented
in the current PR. **No hardware platform is certified or guaranteed to work.**

## Hardware and usage paths

| Hardware | Present status | Required validation |
| --- | --- | --- |
| Intel integrated GPU / Arc | PCI vendor + loaded kernel driver inventory; Mesa graphics packages | Display on Wayland, VA-API/OBS encode, hybrid-GPU laptops |
| AMD Radeon iGPU/dGPU | PCI inventory and open Mesa/firmware baseline | Display, Vulkan, OBS hardware encode, hibernate/record stability |
| NVIDIA GeForce / RTX gaming rigs | Detect PCI adapter and bound driver; do **not** inject proprietary driver automatically | Nouveau vs validated NVIDIA stack, Secure Boot/MOK, mixed GPU, NVENC, suspend |
| USB webcams and audio interfaces | Basic video/ALSA device paths | Stable USB identities, permission, hotplug, PipeWire/OBS routing |
| Gaming capture cards | Basic V4L2 node inventory | Capture reliability, supported formats and real audio/video sync |
| Ethernet / Wi-Fi | Interfaces and wireless flag recorded | Secure interactive Wi-Fi onboarding and offline install |
| NVMe/SATA/USB install targets | ISO work underway | Explicit disk selection, no partition changes until user confirmation |
| VirtualBox/QEMU | Headless Fedora/BitRiver UEFI preview-tested | Display, accelerated graphics, USB passthrough, installer-to-disk |
| BIOS and UEFI motherboards | Detect boot firmware at runtime | ISO currently targets UEFI; BIOS install must be separately tested |
| Laptops and multi-GPU rigs | PCI adapter inventory, not performance certification | Integrated/discrete GPU selection, power management, sleep, batteries |

## Philosophy

- **Use open standards:** Fedora, Linux kernel drivers, Mesa, Wayland,
  PipeWire, OBS, Vulkan/VA-API and ordinary files where practical.
- **Never force proprietary packages** or auto-enroll driver signing keys:
  NVIDIA, codecs and third-party repos need an explicit, documented opt-in.
- **Separate hardware support from use case.** A computer can boot BitRiver,
  run the headless API, or show a desktop without being ready to stream.
- **Flexible modes, shared core:** Broadcast Mode favors reliability;
  Creator Desktop permits general-purpose apps; optional Gaming Mode can
  integrate Steam/Discord after storage, resource and permission boundaries
  are tested. No app bundle is a prerequisite for boot or control plane.
- **No artificial vendor whitelist:** AMD/Intel/NVIDIA are detected, not
  required. Unsupported or unknown devices should report limitations rather
  than prevent headless operation.
- **Do not touch other installed OSes** or silently partition disks.
  Dual-boot, Secure Boot and drive encryption require dedicated testing.

## What the machine inventory means

`bitriver-hw-report` collects CPU/memory, display connector, V4L2 and ALSA
nodes, network interfaces, plus **PCI display adapters, kernel driver names,
UEFI/BIOS status and wireless indicators**. These are local read-only sysfs
queries. They do **not** prove an encoder works or reveal the real GPU inside
a virtual machine. NVIDIA detection is diagnostic, not permission to add RPM
Fusion packages or claim NVENC is supported.

The base image now explicitly installs Fedora `linux-firmware`,
`mesa-dri-drivers` and `mesa-vulkan-drivers`. Those are general-purpose
components; firmware loading, render nodes and driver usability still
depend on the computer, kernel and configuration.

## Compatibility validation matrix before an installable ISO

1. Boot: QEMU BIOS and UEFI, VirtualBox EFI, Intel desktop, AMD desktop,
   NVIDIA gaming desktop, integrated/discrete hybrid laptop.
2. Installer: offline/no-network, NVMe, SATA, single/multiple disks, fresh
   clean install, aborted install, Windows disk present (do not modify),
   recovery when the selected disk is not writable.
3. Graphics: monitor and second monitor, open DRM driver presence, Wayland
   compositor, Chromium kiosk and OBS launch. Test both no GPU and
   no-display headless behavior.
4. Streaming: CPU x264 fallback, VA-API/NVENC when explicitly supported,
   30+ minute recording, USB device replug, network loss and recovery.
5. Desktop mode: separate user session, user-approved software additions,
   game controller/audio/GPU permissions, and no degraded Broadcast Mode.
6. Security: Secure Boot and signing provenance, disk encryption options,
   safe update origin/rollback, no default password on a distributable ISO.

Useful test reports: model of computer/motherboard; CPU and GPU(s); graphics
driver; boot firmware; display(s); Wi-Fi chipset; capture/audio devices;
whether install, first boot and OBS actually worked. **Do not share** serial
numbers, network MAC addresses, personal usernames, stream tokens or keys.

Related: [ISO installer roadmap](ISO_INSTALLER_ROADMAP.md) (on the ISO branch)
and [release gates](RELEASE_READINESS.md).
