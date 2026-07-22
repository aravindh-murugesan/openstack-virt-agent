package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers/iotune"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/hypervisor"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/messaging"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/openstack"
)

// daemonCmd represents the daemon-mode command
var daemonCmd = &cobra.Command{
	Use:   "daemon-mode",
	Short: "Run the virt agent in global daemon mode",
	Long:  `Takes a config file and runs all the enabled controllers' daemon functions.`,
	Run: func(cmd *cobra.Command, args []string) {
		if viper.ConfigFileUsed() == "" {
			fmt.Fprintln(os.Stderr, "Error: No configuration file found. A config file is required for daemon-mode.")
			os.Exit(1)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		setupLogger()

		slog.InfoContext(ctx, "Initializing shared connections...")

		var natsClient *messaging.NATSInstance
		var err error
		if Config.Nats.Enabled {
			natsClient, err = messaging.NewNATSInstance(Config.Nats)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to initialize NATS client", slog.String("error", err.Error()))
				os.Exit(1)
			}
			defer natsClient.Connection.Close()
		}

		osClient, err := openstack.NewOpenstackClient(Config.OpenStack)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize OpenStack client", slog.String("error", err.Error()))
			os.Exit(1)
		}

		libvirtClient, err := hypervisor.NewLibvirtClient("qemu:///system")
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize Libvirt client", slog.String("error", err.Error()))
			os.Exit(1)
		}
		if libvirtClient.Connection != nil {
			defer libvirtClient.Connection.Disconnect()
		}

		connections := &controllers.Connections{
			NatsInstance:      natsClient,
			OpenstackInstance: osClient,
			LibvirtInstance:   libvirtClient,
		}

		var activeControllers []controllers.Controller

		if Config.Controllers.IOPS.Enabled {
			iopsCtrl := iotune.NewIOTuneController(Config.Controllers.IOPS, connections)
			activeControllers = append(activeControllers, iopsCtrl)
		}

		if len(activeControllers) == 0 {
			slog.WarnContext(ctx, "No controllers are enabled in the configuration")
			return
		}

		eg, egCtx := errgroup.WithContext(ctx)

		for _, ctrl := range activeControllers {
			eg.Go(func() error {
				slog.InfoContext(egCtx, "Starting controller", slog.String("controller", ctrl.Name()))
				return ctrl.StartDaemon()
			})
		}

		eg.Go(func() error {
			<-egCtx.Done()
			slog.InfoContext(egCtx, "Shutdown signal received, stopping controllers...")
			for _, ctrl := range activeControllers {
				if err := ctrl.StopDaemon(); err != nil {
					slog.ErrorContext(egCtx, "Failed to stop controller cleanly", slog.String("controller", ctrl.Name()), slog.String("error", err.Error()))
				}
			}
			return nil
		})

		slog.InfoContext(ctx, "Virt Agent daemon is running")
		if err := eg.Wait(); err != nil {
			slog.ErrorContext(ctx, "Daemon exited with error", slog.String("error", err.Error()))
		}
		slog.InfoContext(ctx, "Virt Agent daemon stopped")
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
