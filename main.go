package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func main() {
	cmd := &cli.App{
		Name:   "proxy-dns",
		Action: Run,

		Usage: "Run a DNS over HTTPS proxy server.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "address",
				Usage:   "Listen address for the DNS over HTTPS proxy server.",
				Value:   "localhost",
				EnvVars: []string{"TUNNEL_DNS_ADDRESS"},
			},
			// Note TUN-3758 , we use Int because UInt is not supported with altsrc
			&cli.IntFlag{
				Name:    "port",
				Usage:   "Listen on given port for the DNS over HTTPS proxy server.",
				Value:   53,
				EnvVars: []string{"TUNNEL_DNS_PORT"},
			},
			&cli.StringSliceFlag{
				Name:    "upstream",
				Usage:   "Upstream endpoint URL, you can specify multiple endpoints for redundancy.",
				Value:   cli.NewStringSlice("https://1.1.1.1/dns-query", "https://1.0.0.1/dns-query"),
				EnvVars: []string{"TUNNEL_DNS_UPSTREAM"},
			},
			&cli.StringSliceFlag{
				Name:    "bootstrap",
				Usage:   "bootstrap endpoint URL, you can specify multiple endpoints for redundancy.",
				Value:   cli.NewStringSlice("https://162.159.36.1/dns-query", "https://162.159.46.1/dns-query", "https://[2606:4700:4700::1111]/dns-query", "https://[2606:4700:4700::1001]/dns-query"),
				EnvVars: []string{"TUNNEL_DNS_BOOTSTRAP"},
			},
		},
		ArgsUsage: " ", // can't be the empty string or we get the default output
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cmd.RunContext(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func Run(c *cli.Context) error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}

	listener, err := CreateListener(
		c.String("address"),
		uint16(c.Int("port")),
		c.StringSlice("upstream"),
		c.StringSlice("bootstrap"),
		logger,
	)

	if err != nil {
		logger.Error("Failed to create the listeners", zap.Error(err))
		return err
	}

	// Try to start the server
	readySignal := make(chan struct{})
	err = listener.Start(readySignal)
	if err != nil {
		logger.Error("Failed to start the listeners", zap.Error(err))
		return listener.Stop()
	}
	<-readySignal

	// Wait for signal
	signals := make(chan os.Signal, 10)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	<-signals

	// Shut down server
	err = listener.Stop()
	if err != nil {
		logger.Error("failed to stop", zap.Error(err))
	}
	return err
}
