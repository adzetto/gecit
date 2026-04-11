package dns

import (
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func newTestServer() *Server {
	return &Server{
		ipQueue: make(map[string][]queuedDomain),
		logger:  logrus.New(),
	}
}

func TestPopDomain_Empty(t *testing.T) {
	s := newTestServer()
	if got := s.PopDomain("1.2.3.4"); got != "" {
		t.Fatalf("PopDomain on empty queue: got %q, want \"\"", got)
	}
}

func TestPopDomain_Single(t *testing.T) {
	s := newTestServer()
	s.ipQueue["1.2.3.4"] = []queuedDomain{{domain: "example.com", addedAt: time.Now()}}

	got := s.PopDomain("1.2.3.4")
	if got != "example.com" {
		t.Fatalf("got %q, want %q", got, "example.com")
	}

	// Queue should be deleted after popping the last item.
	if _, exists := s.ipQueue["1.2.3.4"]; exists {
		t.Fatal("queue entry should be deleted after popping last item")
	}
}

func TestPopDomain_FIFO(t *testing.T) {
	s := newTestServer()
	now := time.Now()
	s.ipQueue["10.0.0.1"] = []queuedDomain{
		{domain: "first.com", addedAt: now},
		{domain: "second.com", addedAt: now},
		{domain: "third.com", addedAt: now},
	}

	order := []string{"first.com", "second.com", "third.com"}
	for i, want := range order {
		got := s.PopDomain("10.0.0.1")
		if got != want {
			t.Fatalf("pop %d: got %q, want %q", i, got, want)
		}
	}

	// Queue should be empty now.
	if got := s.PopDomain("10.0.0.1"); got != "" {
		t.Fatalf("queue should be empty, got %q", got)
	}
}

func TestPopDomain_IndependentIPs(t *testing.T) {
	s := newTestServer()
	now := time.Now()
	s.ipQueue["1.1.1.1"] = []queuedDomain{{domain: "cloudflare.com", addedAt: now}}
	s.ipQueue["8.8.8.8"] = []queuedDomain{{domain: "google.com", addedAt: now}}

	got1 := s.PopDomain("1.1.1.1")
	got2 := s.PopDomain("8.8.8.8")

	if got1 != "cloudflare.com" {
		t.Fatalf("IP 1.1.1.1: got %q, want %q", got1, "cloudflare.com")
	}
	if got2 != "google.com" {
		t.Fatalf("IP 8.8.8.8: got %q, want %q", got2, "google.com")
	}
}

func TestPopDomain_Concurrent(t *testing.T) {
	s := newTestServer()

	// Push 1000 domains for one IP.
	domains := make([]string, 1000)
	for i := range domains {
		domains[i] = "domain.com"
	}
	now := time.Now()
	queued := make([]queuedDomain, len(domains))
	for i, domain := range domains {
		queued[i] = queuedDomain{domain: domain, addedAt: now}
	}
	s.ipQueue["10.0.0.1"] = queued

	// Pop from multiple goroutines — must not panic or corrupt.
	var wg sync.WaitGroup
	count := make(chan int, 10)

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := 0
			for {
				if s.PopDomain("10.0.0.1") == "" {
					break
				}
				n++
			}
			count <- n
		}()
	}
	wg.Wait()
	close(count)

	total := 0
	for n := range count {
		total += n
	}
	if total != 1000 {
		t.Fatalf("total pops: got %d, want 1000", total)
	}
}

func TestNewServer_SetsGlobal(t *testing.T) {
	globalDNS = nil
	s := NewServer("cloudflare", logrus.New(), nil)

	if GetDNSServer() != s {
		t.Fatal("GetDNSServer() should return the server set by NewServer()")
	}
}

func TestPopDomain_DropsExpiredEntries(t *testing.T) {
	s := newTestServer()
	s.ipQueue["1.2.3.4"] = []queuedDomain{
		{domain: "stale.com", addedAt: time.Now().Add(-ipQueueEntryTTL - time.Second)},
		{domain: "fresh.com", addedAt: time.Now()},
	}

	got := s.PopDomain("1.2.3.4")
	if got != "fresh.com" {
		t.Fatalf("got %q, want %q", got, "fresh.com")
	}

	if got := s.PopDomain("1.2.3.4"); got != "" {
		t.Fatalf("expected queue to be empty after pruning, got %q", got)
	}
}

func TestPruneQueuedDomains_EmptyAfterExpiry(t *testing.T) {
	q := []queuedDomain{
		{domain: "expired.com", addedAt: time.Now().Add(-ipQueueEntryTTL - time.Second)},
	}

	if got := pruneQueuedDomains(q, time.Now()); got != nil {
		t.Fatalf("expected nil after pruning expired entries, got %#v", got)
	}
}
