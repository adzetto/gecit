# How gecit Bypasses DPI

## The Problem

Some ISPs use Deep Packet Inspection (DPI) to block websites. When you connect to `https://discord.com`, your browser sends a TLS ClientHello that contains the server name (SNI) in plain text:

```
Your PC ──── TLS ClientHello [SNI=discord.com] ────> ISP DPI ──────> Discord
                                                      │
                                                      │ "discord.com is blocked!"
                                                      ╳ Connection dropped
```

The DPI reads the SNI field, matches it against a blocklist, and drops the connection. You see a timeout.

## The Solution: Two-Part Attack

gecit uses two techniques together to bypass the DPI:

### Part 1: Fake ClientHello (DPI Desynchronization)

Before the real TLS handshake, gecit injects a **fake TLS ClientHello** with a different SNI (e.g., `www.google.com`). This packet has a low TTL (Time-To-Live) value so it reaches the DPI but **expires before reaching the server**.

```
Step 1: Fake packet (TTL=8, SNI=google.com)
Your PC ──[FAKE]──> Router ──> DPI ──> Router ──> ... ──> EXPIRED (TTL=0)
                                │
                                │ DPI sees: "google.com, not blocked"
                                │ Records this connection as allowed

Step 2: Real ClientHello (TTL=64, SNI=discord.com)
Your PC ──[REAL]──> Router ──> DPI ──> ... ──> Cloudflare ──> Discord
                                │
                                │ DPI already processed this connection
                                │ Lets it through
```

The DPI processes the fake first, records the connection as going to `google.com` (not blocked), and then lets the real `discord.com` traffic through.

**Why the server never sees the fake**: The fake packet has TTL=8. Each router on the path decrements the TTL by 1. After 8 hops the packet is dropped. Since the DPI is typically 2-4 hops away but the server is 15+ hops away, the fake reaches the DPI but dies long before reaching the server.

**Why the fake must look realistic**: The DPI validates that packets belong to a real TCP connection. The fake must have:
- Correct source/destination IPs and ports
- Correct TCP sequence number (from the real connection)
- Correct TCP ACK number
- Realistic window size

gecit gets all these values from eBPF, which hooks into the kernel's TCP stack.

### Part 2: MSS Fragmentation (ClientHello Splitting)

As a secondary defense, gecit forces the kernel to send the real ClientHello in small TCP segments by setting a low MSS (Maximum Segment Size). Instead of one large packet containing the full SNI, the ClientHello is split across multiple segments.

```
Normal (DPI reads SNI easily):
  Segment 1: [Full ClientHello with SNI=discord.com]  ← DPI reads this

With MSS=40 (fragmented):
  Segment 1: [TLS header...]        ← No SNI here
  Segment 2: [cipher suites...]     ← No SNI here
  Segment 3: [SNI=disc]             ← Partial SNI
  Segment 4: [ord.com...]           ← Rest of SNI
```

The server's TCP stack reassembles the segments correctly. The DPI would need to buffer and reassemble multiple segments to read the SNI — expensive at scale.

## How eBPF Makes This Work

gecit uses eBPF sock_ops to hook into the Linux kernel's TCP stack without any proxy or packet interception:

### Connection Detection
```
App calls connect() to port 443
    → TCP 3-way handshake completes
    → eBPF sock_ops ACTIVE_ESTABLISHED_CB fires
    → gecit knows: source IP, dest IP, ports, TCP seq/ack numbers
```

### MSS Manipulation
```
eBPF calls bpf_setsockopt(TCP_MAXSEG, 40)
    → Kernel's TCP stack will send small segments
    → After 600 bytes, eBPF restores normal MSS (1460)
    → Rest of connection runs at full speed
```

### Fake Injection
```
eBPF emits connection details via perf event
    → Go goroutine receives event
    → Sends fake ClientHello via raw socket (TTL=8, bad checksum)
    → Fake reaches DPI, expires before server
```

## Configuration

```bash
sudo gecit run                        # defaults: MSS=40, TTL=8
sudo gecit run --mss 40 --fake-ttl 8  # explicit
sudo gecit run --fake-ttl 6           # closer DPI (fewer hops)
sudo gecit run --fake-ttl 12          # farther DPI (more hops)
```

### Finding the Right TTL

The fake TTL must be:
- **High enough** to reach the DPI (typically 2-4 hops from your PC)
- **Low enough** to expire before the server (typically 10-20 hops)

Default TTL=8 works for most networks. If it doesn't work:
1. Run `traceroute -n discord.com` to see the hop count
2. The DPI is usually at hop 2-4 (first ISP routers)
3. Set TTL to ~2x the DPI hop count

## Architecture

```
┌──────────┐   ┌──────────────────────┐   ┌────────────┐
│ eBPF     │──>│ Perf Event Buffer    │──>│ Go         │
│ sock_ops │   │ (connection details) │   │ goroutine  │
│          │   └──────────────────────┘   │            │
│ Sets MSS │                              │ Sends fake │
│ per-conn │                              │ via raw    │
│          │                              │ socket     │
└──────────┘                              └────────────┘
     │                                          │
     ▼                                          ▼
┌──────────────────────────────────────────────────────┐
│ Linux Kernel TCP Stack                               │
│ (fragments ClientHello due to small MSS)             │
└──────────────────────────────────────────────────────┘
     │
     ▼
┌──────────┐         ┌──────────┐         ┌──────────┐
│ Fake pkt │         │ Real     │         │ Server   │
│ TTL=8    │────────>│ segments │────────>│ receives │
│ dies at  │  DPI    │ pass     │  DPI    │ real     │
│ hop 8    │ sees it │ through  │ allows  │ data     │
└──────────┘         └──────────┘         └──────────┘
```
