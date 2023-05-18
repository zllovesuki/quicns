package main

import (
	"net"
	"strconv"
	"sync"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/cache"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	LogFieldAddress = "address"
	LogFieldURL     = "url"
)

// Listener is an adapter between CoreDNS server and Warp runnable
type Listener struct {
	server *dnsserver.Server
	wg     sync.WaitGroup
	logger *zap.Logger
}

// Create a CoreDNS server plugin from configuration
func createConfig(address string, port uint16, p plugin.Handler) *dnsserver.Config {
	c := &dnsserver.Config{
		Zone:        ".",
		Transport:   "dns",
		ListenHosts: []string{address},
		Port:        strconv.FormatUint(uint64(port), 10),
	}

	c.AddPlugin(func(next plugin.Handler) plugin.Handler { return p })
	return c
}

// Start blocks for serving requests
func (l *Listener) Start(readySignal chan struct{}) error {
	defer close(readySignal)
	l.logger.Info("Starting DNS over HTTPS proxy server", zap.String(LogFieldAddress, l.server.Address()))

	// Start UDP listener
	if udp, err := l.server.ListenPacket(); err == nil {
		l.wg.Add(1)
		go func() {
			_ = l.server.ServePacket(udp)
			l.wg.Done()
		}()
	} else {
		return errors.Wrap(err, "failed to create a UDP listener")
	}

	// Start TCP listener
	tcp, err := l.server.Listen()
	if err == nil {
		l.wg.Add(1)
		go func() {
			_ = l.server.Serve(tcp)
			l.wg.Done()
		}()
	}

	return errors.Wrap(err, "failed to create a TCP listener")
}

// Stop signals server shutdown and blocks until completed
func (l *Listener) Stop() error {
	if err := l.server.Stop(); err != nil {
		return err
	}

	l.wg.Wait()
	return nil
}

// CreateListener configures the server and bound sockets
func CreateListener(address string, port uint16, upstreams []string, bootstraps []string, logger *zap.Logger) (*Listener, error) {
	// Build the list of upstreams
	upstreamList := make([]Upstream, 0)
	for _, url := range upstreams {
		logger.Info("Adding DNS upstream", zap.String(LogFieldURL, url))
		upstream, err := NewUpstreamHTTPS(url, bootstraps, logger)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create HTTPS upstream")
		}
		upstreamList = append(upstreamList, upstream)
	}

	// Create a local cache with HTTPS proxy plugin
	chain := cache.New()
	chain.Next = ProxyPlugin{
		Upstreams: upstreamList,
	}

	// Format an endpoint
	endpoint := "dns://" + net.JoinHostPort(address, strconv.FormatUint(uint64(port), 10))

	// Create the actual middleware server
	server, err := dnsserver.NewServer(endpoint, []*dnsserver.Config{createConfig(address, port, chain)})
	if err != nil {
		return nil, err
	}

	return &Listener{server: server, logger: logger}, nil
}
