package hardware

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

// bootFirmware reports the mode of the *running* kernel, not the host firmware
// outside a VM. A missing sysfs tree is "unknown", not proof of legacy BIOS.
func bootFirmware(root string) string {
	firmware := filepath.Join(root, "sys", "firmware")
	if _, err := os.Stat(firmware); err != nil {
		return "unknown"
	}
	if _, err := os.Stat(filepath.Join(firmware, "efi")); err == nil {
		return "uefi"
	}
	return "bios"
}

// pciGraphicsDevices reads PCI class 0x03 (display controller). An identified
// vendor, bound kernel driver, or connected monitor does NOT establish
// successful hardware acceleration, OBS compatibility, or encoding capability.
func pciGraphicsDevices(root string) []model.GPUDevice {
	paths, _ := filepath.Glob(filepath.Join(root, "sys", "bus", "pci", "devices", "*"))
	found := make([]model.GPUDevice, 0)
	for _, path := range paths {
		class := readSysfsValue(filepath.Join(path, "class"))
		if !strings.HasPrefix(strings.ToLower(class), "0x03") {
			continue
		}
		vendorID := readSysfsValue(filepath.Join(path, "vendor"))
		deviceID := readSysfsValue(filepath.Join(path, "device"))
		vendor := "other"
		switch strings.ToLower(vendorID) {
		case "0x1002":
			vendor = "amd"
		case "0x8086":
			vendor = "intel"
		case "0x10de":
			vendor = "nvidia"
		}
		found = append(found, model.GPUDevice{
			PCIAddress: filepath.Base(path),
			Vendor: vendor,
			VendorID: vendorID,
			DeviceID: deviceID,
			KernelDriver: driverName(filepath.Join(path, "driver")),
		})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].PCIAddress < found[j].PCIAddress })
	return found
}

// networkAdapters records kernel-reported network interfaces and whether
// wireless sysfs markers exist. This is not connectivity or firmware testing.
func networkAdapters(root string) []model.NetworkAdapter {
	names := networkInterfaces(filepath.Join(root, "sys", "class", "net"))
	adapters := make([]model.NetworkAdapter, 0, len(names))
	for _, name := range names {
		dir := filepath.Join(root, "sys", "class", "net", name)
		_, wifiErr := os.Lstat(filepath.Join(dir, "wireless"))
		_, phyErr := os.Lstat(filepath.Join(dir, "phy80211"))
		adapters = append(adapters, model.NetworkAdapter{
			Name: name,
			Wireless: wifiErr == nil || phyErr == nil,
			KernelDriver: driverName(filepath.Join(dir, "device", "driver")),
		})
	}
	return adapters
}

func readSysfsValue(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func driverName(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	name := filepath.Base(target)
	if name == "." || name == "/" {
		return ""
	}
	return name
}

func compatibilityNotes(report model.HardwareReport) []string {
	notes := make([]string, 0)
	if report.BootFirmware == "bios" {
		notes = append(notes, "Legacy BIOS detected; the experimental installer is currently verified only for UEFI.")
	}
	if !report.DisplayAttached {
		notes = append(notes, "No connected DRM display detected; run headless until graphics are validated.")
	}
	nvidia, unbound, wireless := false, false, false
	for _, gpu := range report.GPUs {
		if gpu.Vendor == "nvidia" {
			nvidia = true
		}
		if gpu.KernelDriver == "" {
			unbound = true
		}
	}
	for _, adapter := range report.NetworkAdapters {
		if adapter.Wireless {
			wireless = true
		}
	}
	if nvidia {
		notes = append(notes, "NVIDIA graphics detected; open/proprietary driver and Secure Boot combinations require separate validation.")
	}
	if unbound {
		notes = append(notes, "At least one GPU has no bound kernel driver; graphical output and encoding are unverified.")
	}
	if wireless {
		notes = append(notes, "Wireless adapter detected; automatic Wi-Fi onboarding is not yet implemented.")
	}
	return notes
}
