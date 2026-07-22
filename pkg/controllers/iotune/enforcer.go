package iotune

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/auth"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/config"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/hypervisor"
	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
)

// EnforceVM iterates through a virtual machine's attached disks and attempts to enforce QoS policies.
func EnforceVM(
	ctx context.Context,
	vm hypervisor.VirtualMachine,
	targetDiskSerial string,
	connections *controllers.Connections,
	cfg *config.IOPSConfig,
	triggeredByNATS bool,
	natsRequestID string,
) {
	reqID := natsRequestID
	if reqID == "" {
		reqID = uuid.NewString()
	}

	// Update context with the VM-specific request ID
	ctx = context.WithValue(ctx, LogRequestID, reqID)

	// Extract contextual values to bootstrap the VM logger
	attrs := []any{
		slog.String(LogVirtualMachineId, vm.UUID),
		slog.String(LogVirtualMachineName, vm.OpenstackName),
		slog.String(LogRequestID, reqID),
	}

	if globalReqID, ok := ctx.Value(LogGlobalReqID).(string); ok {
		attrs = append(attrs, slog.String(LogGlobalReqID, globalReqID))
	}
	if compHandler, ok := ctx.Value(LogControllerComponentHandler).(string); ok {
		attrs = append(attrs, slog.String(LogControllerComponentHandler, compHandler))
	}

	vmLogger := slog.Default().With(attrs...)

	vmLogger.DebugContext(ctx, "Enforcing IO tune policies for VM disks")

	for _, disk := range vm.Devices.Disks {
		// Only Cinder volumes have serial numbers. Skip ephemeral storage.
		if disk.Serial == "" {
			continue
		}

		// If a specific target was requested, skip all other disks.
		if targetDiskSerial != "" && disk.Serial != targetDiskSerial {
			continue
		}

		enforceDisk(ctx, vmLogger, vm, disk, connections, cfg, triggeredByNATS, natsRequestID)
	}
}

func enforceDisk(
	ctx context.Context,
	logger *slog.Logger,
	vm hypervisor.VirtualMachine,
	disk hypervisor.Disk,
	connections *controllers.Connections,
	cfg *config.IOPSConfig,
	triggeredByNATS bool,
	natsRequestID string,
) {
	diskLogger := logger.With(slog.String(LogCinderDiskID, disk.Serial))

	// 1. Fetch Cinder volume metadata
	volume, _, err := connections.OpenstackInstance.GetVolume(ctx, disk.Serial)
	if err != nil {
		diskLogger.ErrorContext(ctx, "Failed to get cinder volume", slog.String(LogMessage, err.Error()))
		return
	}

	// 2. Resolve base limits and max limits based on volume type
	basePolicy, maxPolicy := resolveLimits(ctx, diskLogger, volume, cfg)

	// 3 & 4. Validate intent and determine final policy to apply
	finalIntent, finalPolicyObj, shouldEnforce, isNewlyTrusted := determineFinalPolicy(ctx, diskLogger, volume, basePolicy, maxPolicy, cfg, connections)
	if !shouldEnforce {
		return
	}

	// 5. Apply the finalized policy to the running VM via libvirt
	if err := connections.LibvirtInstance.EnsureConnection(); err != nil {
		diskLogger.ErrorContext(ctx, "Libvirt connection is down, cannot apply IOTune", slog.String(LogMessage, err.Error()))
		// fallback to standard libvirt error handling below by just letting SetVMDiskIOPS fail naturally or handle here
	}

	if err := connections.LibvirtInstance.SetVMDiskIOPS(ctx, vm, finalPolicyObj, disk.Target.Dev); err != nil {
		diskLogger.ErrorContext(ctx, "Failed to enforce IOTune policy via Libvirt", slog.String(LogMessage, err.Error()))

		if triggeredByNATS {
			publishEvent(ctx, connections, natsRequestID, NATSDiskQoSEnforcementNotification{
				ID:           uuid.NewString(),
				RequestID:    natsRequestID,
				InstanceUUID: vm.UUID,
				InstanceName: vm.OpenstackName,
				VolumeUUID:   disk.Serial,
				OccurredAt:   time.Now(),
				Status:       "error",
				Message:      "Failed to apply limits to hypervisor: " + err.Error(),
			})
		}
		return
	}

	diskLogger.InfoContext(ctx, "Successfully enforced IOTune policy for disk", slog.Any("policy", finalPolicyObj))

	// Persist the accepted intent to NATS KV only if we have a valid newly trusted intent
	if finalIntent != nil && isNewlyTrusted {
		lastIntent, _ := GetLastAcceptedIntent(ctx, connections.NatsInstance, disk.Serial)
		if lastIntent == nil || lastIntent.Signature != finalIntent.Signature || lastIntent.Value != finalIntent.Value {
			if err := StoreLastAcceptedIntent(ctx, connections.NatsInstance, disk.Serial, finalIntent); err != nil {
				diskLogger.WarnContext(ctx, "Failed to persist last accepted intent in NATS KV", slog.String(LogMessage, err.Error()))
			}
		} else {
			diskLogger.DebugContext(ctx, "Intent unchanged from last known value, skipping NATS KV update")
		}
	}

	// 6. Publish success event
	if triggeredByNATS {
		publishEvent(ctx, connections, natsRequestID, NATSDiskQoSEnforcementNotification{
			ID:             uuid.NewString(),
			RequestID:      natsRequestID,
			InstanceUUID:   vm.UUID,
			InstanceName:   vm.OpenstackName,
			VolumeUUID:     disk.Serial,
			OccurredAt:     time.Now(),
			Status:         "success",
			EnforcedPolicy: finalPolicyObj,
		})
	}
}

// resolveLimits extracts the base policy and max policy for a given cinder volume based on its volume type.
func resolveLimits(ctx context.Context, logger *slog.Logger, volume volumes.Volume, cfg *config.IOPSConfig) (hypervisor.IOTune, hypervisor.IOTune) {
	basePolicy := mapPolicyToIOTune(cfg.GlobalBasePolicy)
	maxPolicy := basePolicy // Fall back to GlobalBasePolicy as ceiling for unknown volume types

	// Viper automatically lowercases map keys when parsing config files.
	// We must convert the OpenStack volume type to lowercase to match the config map keys.
	normalizedVolType := strings.ToLower(volume.VolumeType)
	if volTypePolicy, ok := cfg.VolumeTypePolicy[normalizedVolType]; ok {
		basePolicy = mapPolicyToIOTune(volTypePolicy.BasePolicy)
		maxPolicy = mapPolicyToIOTune(volTypePolicy.MaxPolicy)
	} else {
		logger.WarnContext(ctx, "Volume type not configured in IOPS policy, falling back to global base policy", slog.String("volume_type", volume.VolumeType))
	}

	return basePolicy, maxPolicy
}

func mapPolicyToIOTune(p config.Policy) hypervisor.IOTune {
	return hypervisor.IOTune{
		TotalIopsSec:     p.TotalIopsSec,
		ReadIopsSec:      p.ReadIopsSec,
		WriteIopsSec:     p.WriteIopsSec,
		TotalBytesSec:    p.TotalBytesSec,
		ReadBytesSec:     p.ReadBytesSec,
		WriteBytesSec:    p.WriteBytesSec,
		TotalIopsSecMax:  p.TotalIopsSecMax,
		ReadIopsSecMax:   p.ReadIopsSecMax,
		WriteIopsSecMax:  p.WriteIopsSecMax,
		TotalBytesSecMax: p.TotalBytesSecMax,
		ReadBytesSecMax:  p.ReadBytesSecMax,
		WriteBytesSecMax: p.WriteBytesSecMax,
		SizeIopsSec:      p.SizeIopsSec,
		GroupName:        p.GroupName,
	}
}

func determineFinalPolicy(
	ctx context.Context,
	logger *slog.Logger,
	volume volumes.Volume,
	basePolicy hypervisor.IOTune,
	maxPolicy hypervisor.IOTune,
	cfg *config.IOPSConfig,
	connections *controllers.Connections,
) (*AcceptedIntent, hypervisor.IOTune, bool, bool) {

	intentStr, metaFound := volume.Metadata[IOPOLICYOVERRIDEKEY]
	signature, sigFound := volume.Metadata[IOPOLICYOVERRIDESIGNATUREKEY]
	requestor, reqFound := volume.Metadata[IOPOLICYINTENTREQUESTOR]

	if !metaFound {
		logger.DebugContext(ctx, "No intent metadata override found. Applying base policy.")
		return nil, basePolicy, true, false
	}

	isTrusted := false
	isNewlyTrusted := false

	// Validate Intent
	if !cfg.ValidateIntent {
		logger.DebugContext(ctx, "Intent validation disabled, trusting the override directly.")
		isTrusted = true
		isNewlyTrusted = true
	} else if sigFound && reqFound {
		pubKey, pubKeyOk := cfg.AuthorizedPubKeys[requestor]
		if pubKeyOk {
			if err := auth.ValidateIOTuneSignature(volume.ID, IOPOLICYOVERRIDEKEY, intentStr, signature, pubKey); err != nil {
				logger.ErrorContext(ctx, "Intent signature validation failed", slog.String(LogMessage, err.Error()))
			} else {
				isTrusted = true
				isNewlyTrusted = true
			}
		} else {
			logger.ErrorContext(ctx, "Intent set by unknown requestor", slog.String("requestor", requestor))
		}
	} else {
		logger.WarnContext(ctx, "Missing intent signature or requestor. Falling back.")
	}

	var acceptedIntent *AcceptedIntent

	// Handle untrusted/invalid intent
	if !isTrusted {
		lastKnown, err := GetLastAcceptedIntent(ctx, connections.NatsInstance, volume.ID)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to fetch last accepted intent", slog.String(LogMessage, err.Error()))
		}

		if lastKnown != nil && lastKnown.Value != "" {
			logger.InfoContext(ctx, "Reverting to last accepted intent from KV store")
			intentStr = lastKnown.Value
			acceptedIntent = lastKnown

			if cfg.ValidateIntent && lastKnown.Signature == "" && lastKnown.Requestor == "" {
				logger.WarnContext(ctx, "Self-healing volume using an unauthenticated intent from the KV store. This may cause continuous validation failures if intent validation is enabled.")
			}

			// Auto-heal cinder metadata with the full cryptographically tied package
			_, _, err = connections.OpenstackInstance.AddVolumeMetadata(ctx, volume.ID, map[string]string{
				IOPOLICYOVERRIDEKEY:          lastKnown.Value,
				IOPOLICYOVERRIDESIGNATUREKEY: lastKnown.Signature,
				IOPOLICYINTENTREQUESTOR:      lastKnown.Requestor,
			})
			if err != nil {
				logger.ErrorContext(ctx, "Failed to self-heal cinder volume metadata", slog.String(LogMessage, err.Error()))
			}
		} else {
			logger.InfoContext(ctx, "No valid last intent. Reverting to base policy.")
			return nil, basePolicy, true, false
		}
	} else {
		acceptedIntent = &AcceptedIntent{
			Value:     intentStr,
			Signature: signature,
			Requestor: requestor,
		}
	}

	// Parse Intent
	parsedPolicy, err := ParseIotuneInput(intentStr, int64(volume.Size))
	if err != nil {
		logger.ErrorContext(ctx, "Failed to parse iotune intent override. Falling back to base policy.", slog.String(LogMessage, err.Error()))
		return nil, basePolicy, true, false
	}

	// Validate against MaxPolicy
	if exceedsLimits(parsedPolicy, maxPolicy) {
		logger.WarnContext(ctx, "Requested IO override exceeds max limits. Applying penalty: falling back to base policy.")
		return nil, basePolicy, true, false
	}

	return acceptedIntent, parsedPolicy, true, isNewlyTrusted
}

// exceedsLimits checks if any metric in `requested` exceeds the corresponding metric in `maxLimits`.
func exceedsLimits(requested, maxLimits hypervisor.IOTune) bool {
	if maxLimits.TotalIopsSec > 0 && requested.TotalIopsSec > maxLimits.TotalIopsSec {
		return true
	}
	if maxLimits.ReadIopsSec > 0 && requested.ReadIopsSec > maxLimits.ReadIopsSec {
		return true
	}
	if maxLimits.WriteIopsSec > 0 && requested.WriteIopsSec > maxLimits.WriteIopsSec {
		return true
	}
	if maxLimits.TotalBytesSec > 0 && requested.TotalBytesSec > maxLimits.TotalBytesSec {
		return true
	}
	if maxLimits.ReadBytesSec > 0 && requested.ReadBytesSec > maxLimits.ReadBytesSec {
		return true
	}
	if maxLimits.WriteBytesSec > 0 && requested.WriteBytesSec > maxLimits.WriteBytesSec {
		return true
	}
	return false
}

// publishEvent is a helper to publish enforcement notifications.
func publishEvent(ctx context.Context, connections *controllers.Connections, natsRequestID string, payload NATSDiskQoSEnforcementNotification) {
	if connections.NatsInstance == nil || connections.NatsInstance.JetStreamConnection == nil {
		return
	}

	data, err := json.Marshal(&payload)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to marshal enforcement notification", slog.String(LogMessage, err.Error()))
		return
	}

	subject := DiskSubjectNotificationPrefix + natsRequestID
	_, err = connections.NatsInstance.JetStreamConnection.Publish(ctx, subject, data)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to publish enforcement notification", slog.String(LogMessage, err.Error()))
	}
}
