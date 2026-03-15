package telemetry

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ProcessSample struct {
	PID          int
	CPUPercent   float64
	RSSBytes     int64
	VirtualBytes int64
	OpenFDs      int
	LogBytes     int64
	ObservedAt   time.Time
}

func SampleOSProcess(pid int, logPath string) (ProcessSample, error) {
	if pid <= 0 {
		return ProcessSample{}, fmt.Errorf("pid must be positive")
	}

	out, err := exec.Command("ps", "-o", "rss=,vsz=,%cpu=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ProcessSample{}, err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return ProcessSample{}, fmt.Errorf("unexpected ps output for pid %d", pid)
	}

	rssKB, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return ProcessSample{}, err
	}
	vszKB, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return ProcessSample{}, err
	}
	cpuPercent, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return ProcessSample{}, err
	}

	logBytes := int64(0)
	if trimmed := strings.TrimSpace(logPath); trimmed != "" {
		if info, err := os.Stat(trimmed); err == nil {
			logBytes = info.Size()
		}
	}

	return ProcessSample{
		PID:          pid,
		CPUPercent:   cpuPercent,
		RSSBytes:     rssKB * 1024,
		VirtualBytes: vszKB * 1024,
		OpenFDs:      sampleOpenFDs(pid),
		LogBytes:     logBytes,
		ObservedAt:   time.Now().UTC(),
	}, nil
}

func sampleOpenFDs(pid int) int {
	if _, err := exec.LookPath("lsof"); err != nil {
		return 0
	}
	cmd := exec.Command("lsof", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := bytes.Count(output, []byte{'\n'})
	if lines <= 1 {
		return 0
	}
	return lines - 1
}
