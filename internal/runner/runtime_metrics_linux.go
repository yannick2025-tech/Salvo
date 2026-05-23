//go:build linux

package runner

import (
	"os"
	"strconv"
	"strings"
)

// sampleSystem fills in Linux-specific system metrics by reading /proc.
func (c *RuntimeMetricsCollector) sampleSystem(s *RuntimeMetricsSnapshot) {
	s.CPUUsagePercent = readCPUUsageLinux()
	s.RSSMemoryMB = readRSSLinux()
	s.ThreadCount = readThreadCountLinux()
}

func readCPUUsageLinux() float64 {
	// Simplified: read total CPU time from /proc/self/stat
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 17 {
		return 0
	}
	// Fields 14-17: utime, stime, cutime, cstime (in clock ticks)
	utime, _ := strconv.ParseFloat(fields[13], 64)
	stime, _ := strconv.ParseFloat(fields[14], 64)
	total := utime + stime
	// This is a cumulative value; proper CPU% requires delta between samples.
	// For now, return 0 as a placeholder until delta tracking is implemented.
	_ = total
	return 0
}

func readRSSLinux() float64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseFloat(fields[1], 64)
				if err == nil {
					return kb / 1024.0
				}
			}
		}
	}
	return 0
}

func readThreadCountLinux() int {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 20 {
		return 0
	}
	n, err := strconv.Atoi(fields[19])
	if err != nil {
		return 0
	}
	return n
}
