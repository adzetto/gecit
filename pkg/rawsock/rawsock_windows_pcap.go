//go:build windows && cgo

package rawsock

import (
	"fmt"
	"net"
	"time"

	"github.com/google/gopacket/pcap"
)

// pcapRawSocket uses Npcap's pcap_sendpacket for fake packet injection.
// Windows blocks raw TCP sockets, so we bypass via the Npcap driver.
type pcapRawSocket struct {
	handle *pcap.Handle
}

func New() (RawSocket, error) {
	iface, err := defaultInterface()
	if err != nil {
		return nil, fmt.Errorf("detect interface: %w", err)
	}

	handle, err := pcap.OpenLive(iface, 0, false, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("pcap open %s: %w (is Npcap installed?)", iface, err)
	}

	return &pcapRawSocket{handle: handle}, nil
}

func (s *pcapRawSocket) SendFake(conn ConnInfo, payload []byte, ttl int) error {
	// Build IP+TCP packet (same as Linux/macOS).
	ipTcp := BuildPacket(conn, payload, ttl)

	// pcap_sendpacket needs an Ethernet frame. Construct a minimal one.
	eth := buildEthernetFrame(ipTcp)

	return s.handle.WritePacketData(eth)
}

func (s *pcapRawSocket) Close() error {
	s.handle.Close()
	return nil
}

func buildEthernetFrame(payload []byte) []byte {
	// Minimal Ethernet frame: dst(6) + src(6) + type(2) + payload
	// Use broadcast dst MAC — the router will accept it.
	frame := make([]byte, 14+len(payload))
	// Dst MAC: broadcast
	for i := 0; i < 6; i++ {
		frame[i] = 0xff
	}
	// Src MAC: zero (will be filled by NIC)
	// EtherType: IPv4 (0x0800)
	frame[12] = 0x08
	frame[13] = 0x00
	copy(frame[14:], payload)
	return frame
}

func defaultInterface() (string, error) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
		return "", fmt.Errorf("pcap find devices: %w (is Npcap installed?)", err)
	}

	for _, dev := range devs {
		for _, addr := range dev.Addresses {
			if ip := addr.IP.To4(); ip != nil && !ip.IsLoopback() {
				// Skip TUN addresses.
				if ip.Equal(net.IPv4(10, 0, 85, 1)) {
					continue
				}
				return dev.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no suitable network interface found")
}
