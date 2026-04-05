package app

import (
	"context"
	"fmt"

	gecitdns "github.com/boratanrikulu/gecit/pkg/dns"
	"github.com/boratanrikulu/gecit/pkg/engine"
	"github.com/boratanrikulu/gecit/pkg/proxy"
	"github.com/boratanrikulu/gecit/pkg/rawsock"
	"github.com/sirupsen/logrus"
)

const proxyPort = 8443

type darwinEngine struct {
	httpProxy  *proxy.HTTPConnectProxy
	sysProxy   *proxy.SystemProxy
	dns        *gecitdns.Server
	dohEnabled bool
	seqTracker *proxy.SeqTracker
	rawSock    rawsock.RawSocket
	logger     *logrus.Logger
}

func newPlatformEngine(cfg engine.Config, logger *logrus.Logger) (engine.Engine, error) {
	rs, err := rawsock.New()
	if err != nil {
		return nil, fmt.Errorf("raw socket: %w", err)
	}

	hp, err := proxy.NewHTTPConnectProxy(proxy.Config{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", proxyPort),
		FakeTTL:    cfg.FakeTTL,
		Ports:      cfg.Ports,
	}, rs, logger)
	if err != nil {
		rs.Close()
		return nil, err
	}

	sysProxy, err := proxy.NewSystemProxy(proxyPort)
	if err != nil {
		rs.Close()
		return nil, fmt.Errorf("system proxy: %w", err)
	}

	iface := cfg.Interface
	if iface == "" {
		iface, _ = proxy.DefaultInterface()
	}
	seqTracker, err := proxy.NewSeqTracker(iface, cfg.Ports)
	if err != nil {
		logger.WithError(err).Warn("seq tracker unavailable")
	}
	proxy.SetSeqTracker(seqTracker)

	dohUpstream := cfg.DoHUpstream
	if dohUpstream == "" {
		dohUpstream = "https://1.1.1.1/dns-query"
	}

	return &darwinEngine{
		httpProxy:  hp,
		sysProxy:   sysProxy,
		dns:        gecitdns.NewServer(dohUpstream, logger),
		dohEnabled: cfg.DoHEnabled,
		seqTracker: seqTracker,
		rawSock:    rs,
		logger:     logger,
	}, nil
}

func (e *darwinEngine) Start(_ context.Context) error {
	// 1. Start DoH DNS (if enabled).
	if e.dohEnabled {
		gecitdns.StopMDNSResponder()
		e.logger.Info("stopped mDNSResponder")

		if err := e.dns.Start(); err != nil {
			gecitdns.ResumeMDNSResponder()
			return err
		}

		if err := gecitdns.SetSystemDNS(e.sysProxy.ServiceName()); err != nil {
			e.dns.Stop()
			gecitdns.ResumeMDNSResponder()
			return err
		}
		e.logger.Info("system DNS set to 127.0.0.1 (DoH)")
	}

	// 2. Start HTTP CONNECT proxy.
	e.logger.WithField("port", proxyPort).Info("starting HTTP CONNECT proxy")
	go e.httpProxy.Serve()

	// 4. Set system HTTPS proxy.
	e.logger.WithField("service", e.sysProxy.ServiceName()).Info("setting system HTTPS proxy")
	if err := e.sysProxy.Start(); err != nil {
		return err
	}

	e.logger.Info("gecit active — DoH DNS + HTTPS proxy + fake injection")
	return nil
}

func (e *darwinEngine) Stop() error {
	e.logger.Info("stopping gecit")
	e.sysProxy.Stop()
	if e.dohEnabled {
		gecitdns.RestoreSystemDNS(e.sysProxy.ServiceName())
		e.dns.Stop()
		gecitdns.ResumeMDNSResponder()
		e.logger.Info("system DNS + mDNSResponder restored")
	}
	e.httpProxy.Stop()
	if e.seqTracker != nil {
		e.seqTracker.Stop()
	}
	if e.rawSock != nil {
		e.rawSock.Close()
	}
	e.logger.Info("gecit stopped — system proxy + DNS restored")
	return nil
}

func (e *darwinEngine) Mode() string { return "http-connect" }
