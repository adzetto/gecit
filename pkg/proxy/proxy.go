package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/boratanrikulu/gecit/pkg/fake"
	"github.com/boratanrikulu/gecit/pkg/rawsock"
	"github.com/sirupsen/logrus"
)

// HTTPConnectProxy is an HTTP CONNECT proxy that injects fake ClientHello
// packets for TLS connections. Apps send "CONNECT host:443 HTTP/1.1",
// the proxy connects to the server, injects a fake, and pipes data.
//
// Set as macOS system HTTPS proxy via networksetup — all apps use it.
type HTTPConnectProxy struct {
	listener    net.Listener
	rawSock     rawsock.RawSocket
	fakeTTL     int
	targetPorts map[uint16]bool
	logger      *logrus.Logger
	done        chan struct{}
}

type Config struct {
	ListenAddr string
	FakeTTL    int
	Ports      []uint16
}

func NewHTTPConnectProxy(cfg Config, rs rawsock.RawSocket, logger *logrus.Logger) (*HTTPConnectProxy, error) {
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", cfg.ListenAddr, err)
	}

	ports := make(map[uint16]bool)
	for _, p := range cfg.Ports {
		ports[p] = true
	}

	ttl := cfg.FakeTTL
	if ttl == 0 {
		ttl = 8
	}

	return &HTTPConnectProxy{
		listener:    ln,
		rawSock:     rs,
		fakeTTL:     ttl,
		targetPorts: ports,
		logger:      logger,
		done:        make(chan struct{}),
	}, nil
}

func (p *HTTPConnectProxy) Serve() error {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.done:
				return nil
			default:
			}
			continue
		}
		go p.handleConn(conn)
	}
}

func (p *HTTPConnectProxy) Stop() error {
	close(p.done)
	p.listener.Close()
	return nil
}

func (p *HTTPConnectProxy) handleConn(clientConn net.Conn) {
	defer clientConn.Close()
	clientConn.SetDeadline(time.Now().Add(10 * time.Second))

	// Read the HTTP CONNECT request.
	// Format: CONNECT discord.com:443 HTTP/1.1\r\nHost: discord.com:443\r\n\r\n
	br := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if req.Method != http.MethodConnect {
		// Not a CONNECT — return 405.
		fmt.Fprintf(clientConn, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
		return
	}

	host := req.Host // e.g., "discord.com:443"

	// Parse port from host.
	_, portStr, err := net.SplitHostPort(host)
	if err != nil {
		fmt.Fprintf(clientConn, "HTTP/1.1 400 Bad Request\r\n\r\n")
		return
	}

	var dstPort uint16
	fmt.Sscanf(portStr, "%d", &dstPort)

	// Connect to the real server.
	serverConn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		fmt.Fprintf(clientConn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer serverConn.Close()

	// Send 200 Connection Established to the client.
	fmt.Fprintf(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	clientConn.SetDeadline(time.Time{}) // clear deadline

	// If target port (443), inject fake before the real ClientHello.
	if p.targetPorts[dstPort] {
		p.injectAndForward(clientConn, serverConn, host, br)
		return
	}

	// Non-target: just pipe.
	pipe(clientConn, serverConn)
}

func (p *HTTPConnectProxy) injectAndForward(clientConn, serverConn net.Conn, dst string, br *bufio.Reader) {
	// Read the TLS ClientHello from the client (held in our buffer).
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// The bufio.Reader may have buffered data from the HTTP request read.
	// Read from it first, then from the raw connection.
	clientHello := make([]byte, 16384)
	n, err := br.Read(clientHello)
	if err != nil {
		return
	}
	clientHello = clientHello[:n]
	clientConn.SetReadDeadline(time.Time{})

	// Get TCP metadata from our server connection.
	serverTCP := serverConn.LocalAddr().(*net.TCPAddr)
	remoteTCP := serverConn.RemoteAddr().(*net.TCPAddr)

	// Get real TCP seq/ack from pcap (captures our SYN-ACK).
	seq, ack := GetSeqAck(serverConn)

	connInfo := rawsock.ConnInfo{
		SrcIP:   serverTCP.IP,
		DstIP:   remoteTCP.IP,
		SrcPort: uint16(serverTCP.Port),
		DstPort: uint16(remoteTCP.Port),
		Seq:     seq,
		Ack:     ack,
	}

	// Send multiple fakes to ensure DPI processes at least one before the real.
	for i := 0; i < 3; i++ {
		p.rawSock.SendFake(connInfo, fake.TLSClientHello, p.fakeTTL)
	}
	p.logger.WithFields(logrus.Fields{
		"dst": dst,
		"seq": seq,
		"ack": ack,
		"ttl": p.fakeTTL,
	}).Info("fake ClientHellos injected")

	// Small delay — let fakes reach the DPI before the real ClientHello.
	time.Sleep(2 * time.Millisecond)

	// Forward real ClientHello to server.
	if _, err := serverConn.Write(clientHello); err != nil {
		return
	}

	// Pipe the rest bidirectionally.
	pipe(clientConn, serverConn)
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(b, a) }()
	go func() { defer wg.Done(); io.Copy(a, b) }()
	wg.Wait()
}
