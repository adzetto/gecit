package capture

import "fmt"

// Linux uses eBPF sock_ops for connection detection, not packet capture.

func NewCapture(iface string, ports []uint16) (Detector, error) {
	return nil, fmt.Errorf("BPF capture not used on Linux (use eBPF sock_ops)")
}
