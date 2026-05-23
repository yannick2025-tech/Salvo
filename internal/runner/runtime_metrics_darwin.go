//go:build darwin

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cpuDeltaState tracks previous CPU time for delta-based instantaneous
// CPU percentage calculation on macOS.
type cpuDeltaState struct {
	mu           sync.Mutex
	prevCPUTime  float64 // cumulative CPU seconds from last sample
	prevTime     time.Time
	initialized  bool
}

var globalCPUState cpuDeltaState

func init() {
	globalCPUState = cpuDeltaState{prevTime: time.Now()}
}

// sampleSystem fills in macOS-specific system metrics using ps.
func (c *RuntimeMetricsCollector) sampleSystem(s *RuntimeMetricsSnapshot) {
	s.CPUUsagePercent = readCPUDeltaDarwin()
	s.RSSMemoryMB = readRSSDarwin()
	s.ThreadCount = readThreadCountDarwin()
}

// readCPUDeltaDarwin computes instantaneous CPU usage using delta
// between two ps samples. This avoids the "cumulative average" problem
// where long-running processes show near-0% CPU.
func readCPUDeltaDarwin() float64 {
	cpuSec, err := readCPUTimeSecondsDarwin()
	if err != nil {
		return 0
	}
	now := time.Now()

	globalCPUState.mu.Lock()
	defer globalCPUState.mu.Unlock()

	if !globalCPUState.initialized {
		globalCPUState.prevCPUTime = cpuSec
		globalCPUState.prevTime = now
		globalCPUState.initialized = true
		return 0
	}

	elapsed := now.Sub(globalCPUState.prevTime).Seconds()
	if elapsed <= 0.01 {
		return 0
	}

	deltaCPU := cpuSec - globalCPUState.prevCPUTime
	if deltaCPU < 0 {
		deltaCPU = 0
	}

	cpuPercent := (deltaCPU / elapsed) * 100.0
	if cpuPercent > 100*float64(getNumCPU()) {
		cpuPercent = 100 * float64(getNumCPU())
	}

	globalCPUState.prevCPUTime = cpuSec
	globalCPUState.prevTime = now

	return cpuPercent
}

// readCPUTimeSecondsDarwin uses ps to get cumulative CPU time for the
// current process, returned as total seconds (user+system).
func readCPUTimeSecondsDarwin() (float64, error) {
	pid := os.Getpid()
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "time=").Output()
	if err != nil {
		return 0, fmt.Errorf("ps time: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	sec, err := parsePSDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", raw, err)
	}
	return sec, nil
}

// parsePSDuration converts macOS ps "time" output format (MM:SS.ss or
// H:MM:SS or days-HH:MM:SS) to float64 seconds.
func parsePSDuration(s string) (float64, error) {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		min, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		sec, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		return min*60 + sec, nil
	case 3:
		hour, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		min, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		sec, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, err
		}
		return hour*3600 + min*60 + sec, nil
	default:
		if idx := strings.Index(s, "-"); idx > 0 {
			days, _ := strconv.ParseFloat(s[:idx], 64)
			rest, err := parsePSDuration(s[idx+1:])
			if err != nil {
				return 0, err
			}
			return days*86400 + rest, nil
		}
		v, err := strconv.ParseFloat(s, 64)
		return v, err
	}
}

func getNumCPU() int {
	n := runtime.NumCPU()
	if n < 1 {
		return 1
	}
	return n
}

func readRSSDarwin() float64 {
	pid := os.Getpid()
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "rss=").Output()
	if err != nil {
		return 0
	}
	kb, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return kb / 1024.0
}

func readThreadCountDarwin() int {
	return 0
}
