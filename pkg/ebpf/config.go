//go:build linux

package ebpf

import (
	"github.com/cilium/ebpf"
)

// gecitConfig must match struct gecit_config_t in maps.h exactly.
// Field order, sizes, and padding must be identical.
type gecitConfig struct {
	MSS               uint16
	RestoreMSS        uint16
	RestoreAfterBytes uint32
	Enabled           uint8
	Reserved          [7]uint8
}

func (m *Manager) pushConfig() error {
	configMap := m.collection.Maps["gecit_config"]
	if configMap == nil {
		return errMapNotFound("gecit_config")
	}

	cfg := gecitConfig{
		MSS:               uint16(m.cfg.MSS),
		RestoreMSS:        uint16(m.cfg.RestoreMSS),
		RestoreAfterBytes: uint32(m.cfg.RestoreAfterBytes),
		Enabled:           1,
	}

	key := uint32(0)
	return configMap.Update(key, cfg, ebpf.UpdateAny)
}

func (m *Manager) pushTargetPorts() error {
	portsMap := m.collection.Maps["target_ports"]
	if portsMap == nil {
		return errMapNotFound("target_ports")
	}

	val := uint8(1)
	for _, port := range m.cfg.Ports {
		if err := portsMap.Update(port, val, ebpf.UpdateAny); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) pushExcludeIPs() error {
	excludeMap := m.collection.Maps["exclude_ips"]
	if excludeMap == nil {
		return nil // map not present in older BPF objects
	}

	val := uint8(1)
	for _, ip := range m.cfg.ExcludeIPs {
		ip4 := ip.To4()
		if ip4 == nil {
			continue
		}
		// Network byte order — same as skops->remote_ip4.
		key := uint32(ip4[0]) | uint32(ip4[1])<<8 | uint32(ip4[2])<<16 | uint32(ip4[3])<<24
		if err := excludeMap.Update(key, val, ebpf.UpdateAny); err != nil {
			return err
		}
	}
	return nil
}

// UpdateEnabled updates the enabled state at runtime without reloading BPF.
func (m *Manager) UpdateEnabled(enabled bool) error {
	configMap := m.collection.Maps["gecit_config"]
	if configMap == nil {
		return errMapNotFound("gecit_config")
	}

	e := uint8(0)
	if enabled {
		e = 1
	}

	cfg := gecitConfig{
		MSS:               uint16(m.cfg.MSS),
		RestoreMSS:        uint16(m.cfg.RestoreMSS),
		RestoreAfterBytes: uint32(m.cfg.RestoreAfterBytes),
		Enabled:           e,
	}

	key := uint32(0)
	return configMap.Update(key, cfg, ebpf.UpdateAny)
}
