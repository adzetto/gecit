package router

// QueueBackend identifies the interception backend used by router mode.
type QueueBackend string

const (
	QueueBackendNFQueue QueueBackend = "nfqueue"
)

// Config is the planned configuration surface for a router-wide mode.
type Config struct {
	WANInterface     string
	LANInterfaces    []string
	TableName        string
	Backend          QueueBackend
	QueueNum         uint16
	PacketMark       uint32
	TCPPorts         []uint16
	UDPPorts         []uint16
	FakeTTL          int
	MaxFlows         int
	ProbeTargets     []string
	AutoHostlistPath string
	EnableQUIC       bool
	EnablePostNAT    bool
}

// DefaultConfig returns conservative defaults for the future NFQUEUE router mode.
func DefaultConfig() Config {
	return Config{
		TableName:     "gecit_router",
		Backend:       QueueBackendNFQueue,
		QueueNum:      200,
		PacketMark:    0x40000000,
		TCPPorts:      []uint16{443},
		UDPPorts:      []uint16{443},
		FakeTTL:       8,
		MaxFlows:      4096,
		ProbeTargets:  []string{"discord.com", "youtube.com"},
		EnableQUIC:    false,
		EnablePostNAT: true,
	}
}
