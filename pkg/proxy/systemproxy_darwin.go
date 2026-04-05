package proxy

import (
	"fmt"
	"os/exec"
	"strings"
)

// SystemProxy manages macOS system HTTPS proxy settings via networksetup.
// All apps that respect system proxy (Safari, Firefox, Chrome, Discord, curl)
// automatically route HTTPS through our proxy. No pf, no anchors.
type SystemProxy struct {
	networkService string
	port           int
}

func NewSystemProxy(port int) (*SystemProxy, error) {
	svc, err := detectNetworkService()
	if err != nil {
		return nil, err
	}
	return &SystemProxy{networkService: svc, port: port}, nil
}

// Start sets the system HTTPS proxy.
func (s *SystemProxy) Start() error {
	// Set HTTPS (secure web) proxy.
	out, err := exec.Command("networksetup", "-setsecurewebproxy",
		s.networkService, "127.0.0.1", fmt.Sprintf("%d", s.port),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("set HTTPS proxy: %s: %w", string(out), err)
	}

	out, err = exec.Command("networksetup", "-setsecurewebproxystate",
		s.networkService, "on",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("enable HTTPS proxy: %s: %w", string(out), err)
	}

	return nil
}

// Stop disables the system HTTPS proxy.
func (s *SystemProxy) Stop() error {
	exec.Command("networksetup", "-setsecurewebproxystate",
		s.networkService, "off").CombinedOutput()
	return nil
}

func (s *SystemProxy) ServiceName() string {
	return s.networkService
}

// DefaultInterface returns the default network interface name.
func DefaultInterface() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "interface:")), nil
		}
	}
	return "", fmt.Errorf("no default interface")
}

// detectNetworkService finds the active network service (e.g., "Wi-Fi").
func detectNetworkService() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("route get default: %w", err)
	}

	var ifaceName string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			ifaceName = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			break
		}
	}
	if ifaceName == "" {
		return "", fmt.Errorf("no default interface found")
	}

	out, err = exec.Command("networksetup", "-listallhardwareports").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list hardware ports: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if strings.Contains(line, "Device: "+ifaceName) && i > 0 {
			return strings.TrimPrefix(strings.TrimSpace(lines[i-1]), "Hardware Port: "), nil
		}
	}

	return "", fmt.Errorf("no network service for %s", ifaceName)
}
