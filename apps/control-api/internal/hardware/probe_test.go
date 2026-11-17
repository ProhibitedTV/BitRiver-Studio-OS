package hardware

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func fixtureFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProbeDetectsHardware(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixtureFile(t, root, "proc/cpuinfo", `model name : Test Processor
processor : 0
processor : 1
processor : 2
processor : 3
`)
	fixtureFile(t, root, "proc/meminfo", "MemTotal:       16777216 kB\n")
	fixtureFile(t, root, "sys/class/drm/card0-HDMI-A-1/status", "connected\n")
	fixtureFile(t, root, "sys/class/drm/card0-DP-1/status", "disconnected\n")
	fixtureFile(t, root, "dev/video2", "")
	fixtureFile(t, root, "dev/video0", "")
	fixtureFile(t, root, "dev/snd/pcmC0D0c", "")
	fixtureFile(t, root, "sys/class/net/eth0/operstate", "up\n")
	fixtureFile(t, root, "sys/class/net/lo/operstate", "unknown\n")

	now := time.Date(2026, 11, 17, 18, 0, 0, 0, time.UTC)
	report := Probe(root, now)
	if !report.DetectedAt.Equal(now) || report.CPUModel != "Test Processor" || report.LogicalCores != 4 || report.MemoryMiB != 16384 {
		t.Fatalf("unexpected CPU/memory report: %#v", report)
	}
	if !report.DisplayAttached || !report.Capabilities.KioskEligible || !report.Capabilities.OBSEligible {
		t.Fatalf("expected attached display and eligible session, got %#v", report)
	}
	if !reflect.DeepEqual(report.VideoDevices, []string{"/dev/video0", "/dev/video2"}) {
		t.Fatalf("unexpected video devices: %#v", report.VideoDevices)
	}
	if !reflect.DeepEqual(report.AudioCaptureDevices, []string{"/dev/snd/pcmC0D0c"}) {
		t.Fatalf("unexpected audio capture devices: %#v", report.AudioCaptureDevices)
	}
	if !reflect.DeepEqual(report.NetworkInterfaces, []string{"eth0"}) {
		t.Fatalf("unexpected interfaces: %#v", report.NetworkInterfaces)
	}
}

func TestProbeHandlesHeadlessMinimalHost(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixtureFile(t, root, "proc/meminfo", "MemTotal:       4194304 kB\n")
	fixtureFile(t, root, "sys/class/drm/card0-HDMI-A-1/status", "disconnected\n")
	report := Probe(root, time.Now())
	if report.DisplayAttached || report.Capabilities.KioskEligible || report.Capabilities.OBSEligible || report.Capabilities.RecommendedForMVP {
		t.Fatalf("headless host should not claim display/OBS readiness: %#v", report)
	}
	if report.MemoryMiB != 4096 || len(report.VideoDevices) != 0 || len(report.AudioCaptureDevices) != 0 || len(report.NetworkInterfaces) != 0 {
		t.Fatalf("unexpected minimal report: %#v", report)
	}
}

func TestProbeIgnoresUnavailableSystemFiles(t *testing.T) {
	t.Parallel()
	report := Probe(t.TempDir(), time.Time{})
	if report.CPUModel != "unknown" || report.LogicalCores != 0 || report.MemoryMiB != 0 ||
		report.DisplayAttached || report.VideoDevices == nil || report.AudioCaptureDevices == nil || report.NetworkInterfaces == nil {
		t.Fatalf("missing system files should not crash or create false devices: %#v", report)
	}
}
