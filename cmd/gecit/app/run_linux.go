package app

import (
	"context"

	gecitdns "github.com/boratanrikulu/gecit/pkg/dns"
	bpf "github.com/boratanrikulu/gecit/pkg/ebpf"
	"github.com/boratanrikulu/gecit/pkg/engine"
	"github.com/sirupsen/logrus"
)

type ebpfEngine struct {
	mgr        *bpf.Manager
	dns        *gecitdns.Server
	dohEnabled bool
	logger     *logrus.Logger
}

func newPlatformEngine(cfg engine.Config, logger *logrus.Logger) (engine.Engine, error) {
	mgr := bpf.NewManager(bpf.Config{
		MSS:               cfg.MSS,
		RestoreMSS:        cfg.RestoreMSS,
		RestoreAfterBytes: cfg.RestoreAfterBytes,
		Ports:             cfg.Ports,
		CgroupPath:        cfg.CgroupPath,
		FakeTTL:           cfg.FakeTTL,
	}, logger)

	dohUpstream := cfg.DoHUpstream
	if dohUpstream == "" {
		dohUpstream = "https://1.1.1.1/dns-query"
	}

	return &ebpfEngine{
		mgr:        mgr,
		dns:        gecitdns.NewServer(dohUpstream, logger),
		dohEnabled: cfg.DoHEnabled,
		logger:     logger,
	}, nil
}

func (e *ebpfEngine) Start(ctx context.Context) error {
	// 1. Start DoH DNS server (if enabled).
	if e.dohEnabled {
		if err := e.dns.Start(); err != nil {
			return err
		}
		if err := gecitdns.SetSystemDNS(); err != nil {
			e.dns.Stop()
			return err
		}
		e.logger.Info("system DNS set to 127.0.0.1 (DoH)")
	}

	// 2. Start eBPF (fake injection + MSS fragmentation).
	if err := e.mgr.Start(ctx); err != nil {
		if e.dohEnabled {
			gecitdns.RestoreSystemDNS()
			e.dns.Stop()
		}
		return err
	}

	return nil
}

func (e *ebpfEngine) Stop() error {
	if e.dohEnabled {
		if err := gecitdns.RestoreSystemDNS(); err != nil {
			e.logger.WithError(err).Warn("failed to restore system DNS")
		}
		if err := e.dns.Stop(); err != nil {
			e.logger.WithError(err).Warn("failed to stop DNS server")
		}
		e.logger.Info("system DNS restored")
	}
	return e.mgr.Stop()
}

func (e *ebpfEngine) Mode() string { return "ebpf-sockops" }
