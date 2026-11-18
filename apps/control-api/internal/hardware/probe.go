package hardware

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

// Probe reads only local kernel/system information. The root argument permits
// deterministic fixture tests without relying on a physical capture device.
func Probe(root string, now time.Time) model.HardwareReport {
	if root == "" {
		root = "/"
	}
	cpuModel, cores := cpuInfo(filepath.Join(root, "proc", "cpuinfo"))
	if cores == 0 && filepath.Clean(root) == "/" {
		cores = runtime.NumCPU()
	}
	memory := memoryMiB(filepath.Join(root, "proc", "meminfo"))
	displayAttached := hasDisplay(filepath.Join(root, "sys", "class", "drm"))

	report := model.HardwareReport{
		DetectedAt:          now.UTC(),
		Architecture:        runtime.GOARCH,
		CPUModel:            cpuModel,
		LogicalCores:        cores,
		MemoryMiB:           memory,
		DisplayAttached:     displayAttached,
		BootFirmware:        bootFirmware(root),
		GPUs:                pciGraphicsDevices(root),
		NetworkAdapters:     networkAdapters(root),
		VideoDevices:        devices(root, "dev/video[0-9]*"),
		AudioCaptureDevices: devices(root, "dev/snd/pcmC*D*c"),
		NetworkInterfaces:   networkInterfaces(filepath.Join(root, "sys", "class", "net")),
	}
	// These are resource heuristics, not hardware certification. GPU/encoder
	// capability and capture reliability still require real appliance testing.
	report.CompatibilityNotes = compatibilityNotes(report)
	report.Capabilities = model.HardwareCapabilities{
		KioskEligible:     displayAttached,
		OBSEligible:       displayAttached && memory >= 4096,
		RecommendedForMVP: runtime.GOARCH == "amd64" && displayAttached && memory >= 16384 && cores >= 4,
	}
	return report
}

func cpuInfo(path string) (string, int) {
	file, err := os.Open(path)
	if err != nil {
		return "unknown", 0
	}
	defer file.Close()

	modelName := "unknown"
	cores := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "model name", "hardware":
			if modelName == "unknown" && value != "" {
				modelName = value
			}
		case "processor":
			if _, err := strconv.Atoi(value); err == nil {
				cores++
			} else if modelName == "unknown" && value != "" {
				modelName = value
			}
		}
	}
	return modelName, cores
}

func memoryMiB(path string) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kib, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return int(kib / 1024)
			}
		}
	}
	return 0
}

func hasDisplay(drmDir string) bool {
	entries, err := filepath.Glob(filepath.Join(drmDir, "*", "status"))
	if err != nil {
		return false
	}
	for _, path := range entries {
		content, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(content)) == "connected" {
			return true
		}
	}
	return false
}

func devices(root string, pattern string) []string {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		relative, err := filepath.Rel(root, match)
		if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
			continue
		}
		out = append(out, "/"+filepath.ToSlash(relative))
	}
	sort.Strings(out)
	return out
}

func networkInterfaces(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() != "lo" {
			out = append(out, entry.Name())
		}
	}
	sort.Strings(out)
	return out
}
