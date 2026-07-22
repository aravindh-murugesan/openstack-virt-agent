package iotune

import (
	"context"
	"log/slog"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/config"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers"
	"golang.org/x/sync/errgroup"
)

type IOTuneController struct {
	cfg         config.IOPSConfig
	connections *controllers.Connections
	cancel      context.CancelFunc
}

func NewIOTuneController(cfg config.IOPSConfig, connections *controllers.Connections) *IOTuneController {
	return &IOTuneController{
		cfg:         cfg,
		connections: connections,
	}
}

func (c *IOTuneController) Name() string {
	return "iotune-controller"
}

func (c *IOTuneController) IsEnabled() bool {
	return c.cfg.Enabled
}

func (c *IOTuneController) StartDaemon() error {
	slog.Info("Starting iotune-controller daemon...")

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	eg, egCtx := errgroup.WithContext(ctx)

	// 1. Start Worker Pool
	// In Go, it's safe for multiple writers to write to a channel, but only the creator should close it.
	// Since we are using an errgroup, we won't close the channel explicitly; it will be garbage collected
	// when the context cancels and routines exit.
	jobQueue := StartWorkerPool(egCtx, 5, c.connections, &c.cfg) // Hardcoded to 3 workers for now

	// 2. Start Audit Loop
	eg.Go(func() error {
		StartAuditLoop(egCtx, c.connections, &c.cfg, jobQueue)
		return nil
	})

	// 3. Start Libvirt Listener
	eg.Go(func() error {
		StartLibvirtListener(egCtx, c.connections, jobQueue)
		return nil
	})

	// 4. Start NATS Listener
	eg.Go(func() error {
		StartNATSListener(egCtx, c.connections, jobQueue)
		return nil
	})

	return eg.Wait()
}

func (c *IOTuneController) StopDaemon() error {
	slog.Info("Stopping iotune-controller daemon...")
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}
