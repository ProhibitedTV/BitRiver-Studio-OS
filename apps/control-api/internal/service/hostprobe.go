package service

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func collectHostMetrics(recordingPath string, setup model.SetupStatus) (model.CPUStatus, model.MemoryStatus, model.NetworkSummary) {
	cpu := model.CPUStatus{
		Model:        fallbackString(setup.Hardware.CPUModel, runtime.GOARCH),
		LogicalCores: maxInt(runtime.NumCPU(), setup.Hardware.LogicalCores),
		UsagePercent: cpuUsagePercent(),
		LoadAverage:  loadAverage(),
	}

	totalMiB, freeMiB := memoryMiB()
	memory := model.MemoryStatus{
		TotalMiB: totalMiB,
		FreeMiB:  freeMiB,
		UsedMiB:  maxInt(totalMiB-freeMiB, 0),
	}

	interfaces := setup.Hardware.NetworkInterfaces
	if len(interfaces) == 0 {
		interfaces = []string{"eth0"}
	}

	return cpu, memory, model.NetworkSummary{
		LANAddress: firstLANAddress(),
		Hostname:   fallbackString(setup.Hostname, "bitriver.local"),
		Interfaces: interfaces,
	}
}

func collectDiskStatus(recordingPath string) model.DiskStatus {
	path := recordingsDir(recordingPath)
	if runtime.GOOS != "linux" {
		return model.DiskStatus{
			Path:      path,
			TotalGb:   0,
			UsedGb:    0,
			FreeGb:    0,
			Available: fileExists(path),
		}
	}

	if err := os.MkdirAll(path, 0o750); err != nil {
		return model.DiskStatus{
			Path:      path,
			Available: false,
			Warning:   err.Error(),
		}
	}

	total, used, free, err := diskUsage(path)
	if err != nil {
		return model.DiskStatus{
			Path:      path,
			Available: false,
			Warning:   err.Error(),
		}
	}
	warning := ""
	if free < 25 {
		warning = "recording volume is below 25 GB free"
	}

	return model.DiskStatus{
		Path:      path,
		TotalGb:   total,
		UsedGb:    used,
		FreeGb:    free,
		Warning:   warning,
		Available: true,
	}
}

func cpuUsagePercent() float64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	idleA, totalA, ok := cpuSample()
	if !ok {
		return 0
	}
	time.Sleep(50 * time.Millisecond)
	idleB, totalB, ok := cpuSample()
	if !ok || totalB <= totalA {
		return 0
	}

	idleDelta := idleB - idleA
	totalDelta := totalB - totalA
	return 100 * (1 - float64(idleDelta)/float64(totalDelta))
}

func cpuSample() (uint64, uint64, bool) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, 0, false
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 {
		return 0, 0, false
	}

	var total uint64
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		values = append(values, value)
		total += value
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return idle, total, true
}

func loadAverage() float64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	payload, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(payload))
	if len(fields) == 0 {
		return 0
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return value
}

func memoryMiB() (int, int) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}
	payload, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}

	total := 0
	available := 0
	for _, line := range strings.Split(string(payload), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = kibToMiB(fields[1])
		case "MemAvailable:":
			available = kibToMiB(fields[1])
		}
	}
	return total, available
}

func firstLANAddress() string {
	if runtime.GOOS != "linux" {
		return "127.0.0.1"
	}
	payload, err := os.ReadFile("/proc/net/fib_trie")
	if err != nil {
		return "127.0.0.1"
	}

	for _, line := range strings.Split(string(payload), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Count(trimmed, ".") == 3 && !strings.HasPrefix(trimmed, "127.") {
			return strings.TrimPrefix(trimmed, "|-- ")
		}
	}

	return "127.0.0.1"
}

func kibToMiB(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed / 1024
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
