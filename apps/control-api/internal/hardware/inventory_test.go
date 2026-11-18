package hardware

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixtureDriver(t *testing.T, root, link, name string) {
	t.Helper()
	path := filepath.Join(root, link)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/sys/bus/pci/drivers/"+name, path); err != nil {
		t.Fatal(err)
	}
}

func TestMixedGamingPCInventory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sys/firmware/efi"), 0755); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, root, "proc/cpuinfo", "model name : Test CPU\nprocessor : 0\nprocessor : 1\nprocessor : 2\nprocessor : 3\n")
	fixtureFile(t, root, "proc/meminfo", "MemTotal: 33554432 kB\n")
	fixtureFile(t, root, "sys/class/drm/card0-HDMI-A-1/status", "connected\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:00:02.0/class", "0x030000\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:00:02.0/vendor", "0x8086\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:00:02.0/device", "0x9a49\n")
	fixtureDriver(t, root, "sys/bus/pci/devices/0000:00:02.0/driver", "i915")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:01:00.0/class", "0x030200\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:01:00.0/vendor", "0x10de\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:01:00.0/device", "0x2484\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:00:1f.6/class", "0x020000\n") // not a GPU
	fixtureFile(t, root, "sys/class/net/enp1s0/operstate", "up\n")
	fixtureDriver(t, root, "sys/class/net/enp1s0/device/driver", "e1000e")
	fixtureFile(t, root, "sys/class/net/wlp2s0/wireless/placeholder", "")
	fixtureDriver(t, root, "sys/class/net/wlp2s0/device/driver", "iwlwifi")
	report := Probe(root, time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC))
	if report.BootFirmware != "uefi" || len(report.GPUs) != 2 {
		t.Fatalf("unexpected firmware/GPUs: %#v", report)
	}
	if report.GPUs[0].Vendor != "intel" || report.GPUs[0].KernelDriver != "i915" ||
		report.GPUs[1].Vendor != "nvidia" || report.GPUs[1].KernelDriver != "" {
		t.Fatalf("unexpected PCI graphics: %#v", report.GPUs)
	}
	if !reflect.DeepEqual(report.NetworkAdapters[0].Name, "enp1s0") ||
		report.NetworkAdapters[0].Wireless || !report.NetworkAdapters[1].Wireless {
		t.Fatalf("network adapter discovery failed: %#v", report.NetworkAdapters)
	}
	notes := strings.Join(report.CompatibilityNotes, " ")
	if !strings.Contains(notes, "NVIDIA") || !strings.Contains(notes, "no bound kernel driver") ||
		!strings.Contains(notes, "Wi-Fi onboarding") {
		t.Fatalf("hardware caution notes missing: %q", notes)
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("hardware JSON could not be encoded: %v", err)
	}
}

func TestAMDBIOSAndHeadlessHardware(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sys/firmware"), 0755); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, root, "sys/bus/pci/devices/0000:02:00.0/class", "0x030000\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:02:00.0/vendor", "0x1002\n")
	fixtureFile(t, root, "sys/bus/pci/devices/0000:02:00.0/device", "0x73bf\n")
	fixtureDriver(t, root, "sys/bus/pci/devices/0000:02:00.0/driver", "amdgpu")
	report := Probe(root, time.Now())
	if report.BootFirmware != "bios" || len(report.GPUs) != 1 ||
		report.GPUs[0].Vendor != "amd" || report.GPUs[0].KernelDriver != "amdgpu" {
		t.Fatalf("unexpected AMD/BIOS report: %#v", report)
	}
	if report.Capabilities.OBSEligible || report.Capabilities.RecommendedForMVP {
		t.Fatal("GPU presence alone must not claim supported streaming performance")
	}
	notes := strings.Join(report.CompatibilityNotes, " ")
	if !strings.Contains(notes, "Legacy BIOS") || !strings.Contains(notes, "headless") {
		t.Fatalf("missing limitations: %q", notes)
	}
}

func TestUnavailableSysfsIsNotCertified(t *testing.T) {
	t.Parallel()
	report := Probe(t.TempDir(), time.Now())
	if report.BootFirmware != "unknown" || report.GPUs == nil ||
		report.NetworkAdapters == nil || report.CompatibilityNotes == nil ||
		len(report.GPUs) != 0 || report.Capabilities.OBSEligible {
		t.Fatalf("unavailable sysfs should produce safe, empty inventory: %#v", report)
	}
}
