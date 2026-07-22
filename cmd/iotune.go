package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/auth"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers/iotune"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/messaging"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/openstack"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	volumeID       string
	totalIops      uint64
	writeIops      uint64
	readIops       uint64
	privateKeyFile string
)

// iotuneCmd represents the iotune-controller command
var iotuneCmd = &cobra.Command{
	Use:   "iotune-controller",
	Short: "Controller Specific Operations for iotune",
	Long:  `Controller specific commands for iotune-controller.`,
}

var applyIntentCmd = &cobra.Command{
	Use:   "apply-intent",
	Short: "Submit a QoS override intent for a volume",
	Long:  `Applies an I/O policy override intent to a Cinder volume, triggers enforcement, and waits for a response.`,
	Run: func(cmd *cobra.Command, args []string) {
		if viper.ConfigFileUsed() == "" {
			fmt.Fprintln(os.Stderr, "Error: No configuration file found. A config file is required.")
			os.Exit(1)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		setupLogger()

		if !Config.Nats.Enabled {
			slog.ErrorContext(ctx, "NATS is disabled in configuration. Cannot apply intent via NATS.")
			os.Exit(1)
		}

		// Connect to NATS
		natsClient, err := messaging.NewNATSInstance(Config.Nats)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize NATS client", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer natsClient.Connection.Close()

		// Connect to OpenStack
		osClient, err := openstack.NewOpenstackClient(Config.OpenStack)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize OpenStack client", slog.String("error", err.Error()))
			os.Exit(1)
		}

		if totalIops > 0 && (readIops > 0 || writeIops > 0) {
			fmt.Fprintln(os.Stderr, "Error: cannot specify read/write IOPS when total IOPS is specified.")
			os.Exit(1)
		}
		if totalIops == 0 && (readIops == 0 || writeIops == 0) {
			fmt.Fprintln(os.Stderr, "Error: when total IOPS is 0, both read and write IOPS must be specified.")
			os.Exit(1)
		}

		// Prepare Metadata
		intentStr := fmt.Sprintf("%d,%d,%d", totalIops, writeIops, readIops)

		metadata := map[string]string{
			iotune.IOPOLICYOVERRIDEKEY: intentStr,
		}

		// Handle Private Key and Signing
		if privateKeyFile != "" {
			keyBytes, err := os.ReadFile(privateKeyFile)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to read private key file", slog.String("error", err.Error()))
				os.Exit(1)
			}

			var privateKeyB64 string
			block, _ := pem.Decode(keyBytes)
			if block != nil {
				// If it's a PEM file, extract the bytes and re-encode to clean base64
				privateKeyB64 = base64.StdEncoding.EncodeToString(block.Bytes)
			} else {
				// Fallback: assume it's a raw base64 string, just trim whitespace/newlines
				privateKeyB64 = strings.TrimSpace(string(keyBytes))
			}

			requestor, err := auth.MatchPrivateKeyToRequestor(privateKeyB64, Config.Controllers.IOPS.AuthorizedPubKeys)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to match private key to authorized requestor", slog.String("error", err.Error()))
				os.Exit(1)
			}

			signature, err := auth.CreateIOTuneSignature(volumeID, iotune.IOPOLICYOVERRIDEKEY, intentStr, privateKeyB64)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to create signature", slog.String("error", err.Error()))
				os.Exit(1)
			}

			metadata[iotune.IOPOLICYINTENTREQUESTOR] = requestor
			metadata[iotune.IOPOLICYOVERRIDESIGNATUREKEY] = signature
		}

		// Write to OpenStack
		slog.InfoContext(ctx, "Updating volume metadata in OpenStack...", slog.String("volume_id", volumeID))
		updatedVol, _, err := osClient.AddVolumeMetadata(ctx, volumeID, metadata)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to update OpenStack volume metadata", slog.String("error", err.Error()))
			os.Exit(1)
		}

		if len(updatedVol.Attachments) == 0 {
			slog.InfoContext(ctx, "Volume is not attached to any instance. Metadata updated, but skipping NATS trigger.", slog.String("volume_id", volumeID))
			return
		}

		instanceID := updatedVol.Attachments[0].ServerID

		// Generate RequestID and set up listener
		requestID := uuid.NewString()
		replySubject := iotune.DiskSubjectNotificationPrefix + requestID

		sub, err := natsClient.Connection.SubscribeSync(replySubject)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to subscribe to response subject", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer sub.Unsubscribe()

		// Publish intent trigger event
		eventData := map[string]string{
			"volume_uuid":   volumeID,
			"request_id":    requestID,
			"instance_uuid": instanceID,
		}
		eventBytes, _ := json.Marshal(eventData)

		slog.InfoContext(ctx, "Publishing NATS trigger event...", slog.String("subject", iotune.DiskSubjectSubscription))
		if err := natsClient.Connection.Publish(iotune.DiskSubjectSubscription, eventBytes); err != nil {
			slog.ErrorContext(ctx, "Failed to publish NATS trigger event", slog.String("error", err.Error()))
			os.Exit(1)
		}

		// Wait for response
		slog.InfoContext(ctx, "Waiting up to 2 minutes for enforcement response...", slog.String("request_id", requestID))
		msg, err := sub.NextMsg(2 * time.Minute)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to receive response from daemon (timed out)", slog.String("error", err.Error()))
			os.Exit(1)
		}

		slog.InfoContext(ctx, "Received enforcement response", slog.String("data", string(msg.Data)))
	},
}

func init() {
	rootCmd.AddCommand(iotuneCmd)
	iotuneCmd.AddCommand(applyIntentCmd)

	applyIntentCmd.Flags().StringVar(&volumeID, "volume-id", "", "Cinder Volume ID")
	applyIntentCmd.MarkFlagRequired("volume-id")

	applyIntentCmd.Flags().Uint64Var(&totalIops, "total-iops", 0, "Total IOPS value to apply")
	applyIntentCmd.Flags().Uint64Var(&writeIops, "write-iops", 0, "Write IOPS value to apply")
	applyIntentCmd.Flags().Uint64Var(&readIops, "read-iops", 0, "Read IOPS value to apply")
	applyIntentCmd.MarkFlagRequired("total-iops")

	applyIntentCmd.Flags().StringVar(&privateKeyFile, "private-key-file", "", "Path to the base64-encoded Ed25519 private key")
}
