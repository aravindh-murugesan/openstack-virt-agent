package iotune

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/hypervisor"
)

// ParseIotuneInput parses the provided JSON string representing IO policy overrides
// and returns a validated IOTune structure.
func ParseIotuneInput(input string, volSize int64) (hypervisor.IOTune, error) {
	var inputIotune hypervisor.IOTune

	if input == "" {
		return inputIotune, fmt.Errorf("metadata for volume indicates no policy set")
	}

	parts := strings.Split(input, ",")
	
	// Pad parts up to 3 elements for backward compatibility
	for len(parts) < 3 {
		parts = append(parts, "0")
	}

	totalIops, _ := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
	writeIops, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
	readIops, _ := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64)

	// Logical constraints enforcement per user request
	if totalIops > 0 {
		// If total is specified, strictly ignore read/write
		writeIops = 0
		readIops = 0
	}

	inputIotune.TotalIopsSec = totalIops
	inputIotune.WriteIopsSec = writeIops
	inputIotune.ReadIopsSec = readIops

	return inputIotune, nil
}
