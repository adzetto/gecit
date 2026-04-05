# gecit — Project Overview

## Project Summary

gecit is a cross-platform DPI bypass tool. It injects fake TLS ClientHello packets to desynchronize Deep Packet Inspection middleboxes, and includes a built-in DoH DNS resolver to bypass DNS poisoning.

**Linux**: eBPF sock_ops hooks into the kernel TCP stack — no proxy, no traffic redirection.
**macOS**: HTTP CONNECT proxy with system-wide proxy configuration — all apps automatically use it.

Both platforms include a built-in DoH (DNS-over-HTTPS) server that bypasses DNS poisoning.

## How It Works

See [HOW_IT_WORKS.md](HOW_IT_WORKS.md) for the full technical explanation.

**Short version**: Before the real TLS ClientHello is sent, gecit injects a fake one with a different SNI (e.g., `www.google.com`) and a low TTL. The DPI processes the fake and records the connection as allowed. The fake packet expires before reaching the server. The real ClientHello passes through because the DPI already made its decision.

## Project Structure

```
cmd/gecit/
├── app/
│   ├── root.go              # CLI root command + global flags
│   ├── run.go               # `gecit run` — starts the engine
│   ├── run_linux.go          # Linux: eBPF + DoH DNS
│   ├── run_darwin.go         # macOS: HTTP CONNECT proxy + DoH DNS
│   ├── run_windows.go        # Windows: placeholder
│   ├── status.go             # `gecit status`
│   └── status_{platform}.go  # Platform-specific status
│
pkg/
├── ebpf/                     # Linux eBPF implementation
│   ├── manager.go            # BPF loader, perf event reader, fake injection
│   ├── config.go             # Go↔BPF config map push
│   ├── features.go           # Runtime kernel capability detection
│   ├── amd64.go / arm64.go   # go:embed pre-compiled BPF objects
│   └── bpf/
│       ├── sockops.bpf.c     # CO-RE sock_ops BPF program
│       ├── maps.h            # BPF map definitions
│       └── bpf.mk            # BPF compilation (clang/zig)
│
├── proxy/                    # macOS HTTP CONNECT proxy
│   ├── proxy.go              # CONNECT proxy + fake injection
│   ├── systemproxy_darwin.go # System proxy + DNS via networksetup
│   └── seqtracker_darwin.go  # pcap-based TCP seq/ack extraction
│
├── dns/                      # DoH DNS resolver (both platforms)
│   ├── server.go             # Local DNS server (127.0.0.1:53)
│   ├── doh.go                # DoH client (RFC 8484)
│   ├── system_linux.go       # /etc/resolv.conf management
│   └── system_darwin.go      # networksetup DNS + mDNSResponder management
│
├── rawsock/                  # Raw socket for fake packet injection
│   ├── rawsock.go            # Shared packet builder + checksum
│   ├── rawsock_{platform}.go # Platform-specific socket creation
│   └── ipheader_{platform}.go # IP header byte order
│
├── fake/                     # Fake TLS ClientHello payload
│   └── clienthello.go        # Pre-built ClientHello (SNI=www.google.com)
│
├── engine/                   # Platform-agnostic interfaces
│   ├── engine.go             # Engine interface (Start/Stop/Mode)
│   └── config.go             # Shared configuration
│
└── capture/                  # Packet capture (macOS pcap)
    ├── capture.go            # Detector interface
    ├── capture_darwin.go     # pcap SYN-ACK capture
    └── capture_linux.go      # Stub (Linux uses eBPF)
```

## Building

```bash
# Linux (requires Lima VM on macOS for BPF compilation)
lima make                          # BPF + both Linux architectures
lima make gecit-linux-arm64        # arm64 only

# macOS
make gecit-darwin-arm64

# BPF objects only
lima make bpf-all
```

## Running

```bash
# Linux
sudo ./bin/gecit-linux-arm64 run

# macOS
sudo ./bin/gecit-darwin-arm64 run

# Custom settings
sudo gecit run --fake-ttl 12 --doh https://8.8.8.8/dns-query
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--mss` | 40 | TCP MSS for ClientHello fragmentation |
| `--fake-ttl` | 8 | TTL for fake packets (must reach DPI but not server) |
| `--doh` | `https://1.1.1.1/dns-query` | DoH upstream URL |
| `--ports` | 443 | Target destination ports |
| `--interface` | auto | Network interface |
| `--restore-after-bytes` | 600 | Restore normal MSS after N bytes (Linux) |
| `--cgroup` | `/sys/fs/cgroup` | Cgroup v2 path (Linux) |

## Platform Details

### Linux (eBPF)

- BPF sock_ops program attaches to cgroup v2
- Sets TCP_MAXSEG on port 443 connections (kernel auto-fragments)
- Reads `snd_nxt`/`rcv_nxt` from kernel socket struct for fake packet metadata
- Perf events notify userspace of new connections
- Raw socket sends fake ClientHello with low TTL
- MSS restored after handshake via WRITE_HDR_OPT_CB
- Kernel requirement: 5.10+ (optimal), 5.x+ (minimum)

### macOS (HTTP CONNECT Proxy)

- HTTP CONNECT proxy on `127.0.0.1:8443`
- System HTTPS proxy set via `networksetup` (all apps use it automatically)
- pcap captures SYN-ACKs for TCP seq/ack extraction
- Raw socket sends fake ClientHello with low TTL
- mDNSResponder temporarily stopped for port 53 DNS server

### DoH DNS (Both Platforms)

- Local UDP DNS server on `127.0.0.1:53`
- Forwards all queries via DNS-over-HTTPS (RFC 8484 wire format)
- Default upstream: Cloudflare `1.1.1.1`
- System DNS changed on start, restored on stop
- Breadcrumb file for crash recovery (macOS: `/tmp/gecit-dns-backup`)
