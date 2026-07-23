package iotune

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/hypervisor"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/messaging"
	"github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// StartLibvirtListener subscribes to DomainEventIDDeviceAdded and DomainEventIDLifecycle events.
func StartLibvirtListener(ctx context.Context, connections *controllers.Connections, jobQueue chan<- EnforcementJob) {
	slog.InfoContext(ctx, "Libvirt event listener started")

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Libvirt event listener stopped")
			return
		default:
		}

		if err := connections.LibvirtInstance.EnsureConnection(); err != nil {
			slog.ErrorContext(ctx, "Libvirt connection down, listener retrying...", slog.String(LogMessage, err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		diskEvents, err := connections.LibvirtInstance.Connection.SubscribeEvents(ctx, libvirt.DomainEventIDDeviceAdded, libvirt.OptDomain{})
		if err != nil {
			slog.ErrorContext(ctx, "Error subscribing to libvirt disk events", slog.String(LogMessage, err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		lifecycleEvents, err := connections.LibvirtInstance.Connection.SubscribeEvents(ctx, libvirt.DomainEventIDLifecycle, libvirt.OptDomain{})
		if err != nil {
			slog.ErrorContext(ctx, "Error subscribing to libvirt lifecycle events", slog.String(LogMessage, err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		err = processLibvirtEvents(ctx, connections, diskEvents, lifecycleEvents, jobQueue)
		if err != nil {
			slog.WarnContext(ctx, "Libvirt event stream closed, reconnecting...", slog.String(LogMessage, err.Error()))
			time.Sleep(2 * time.Second)
		}
	}
}

func processLibvirtEvents(
	ctx context.Context,
	connections *controllers.Connections,
	diskEvents <-chan interface{},
	lifecycleEvents <-chan interface{},
	jobQueue chan<- EnforcementJob,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-diskEvents:
			if !ok {
				return fmt.Errorf("disk events channel closed")
			}
			msg, ok := ev.(*libvirt.DomainEventCallbackDeviceAddedMsg)
			if ok {
				slog.DebugContext(
					ctx, "Disk attachment event detected",
					slog.String("dev_alias", msg.DevAlias),
					slog.String(LogVirtualMachineId, msg.Dom.Name),
				)
				enqueueVMByName(ctx, connections, msg.Dom.Name, "", false, "", jobQueue)
			}
		case ev, ok := <-lifecycleEvents:
			if !ok {
				return fmt.Errorf("lifecycle events channel closed")
			}
			msg, ok := ev.(*libvirt.DomainEventCallbackLifecycleMsg)
			if ok {
				eventKey, details := hypervisor.ParseLifecycle(msg.Msg.Event, msg.Msg.Detail)
				if eventKey == "VM_STARTED" || (eventKey == "VM_RESUMED" && strings.Contains(details, "Migration Finished")) {

					// Ignore incoming live migration, as this will fail anyway
					if strings.Contains(details, "Incoming Migration") {
						slog.DebugContext(
							ctx, "Ignoring incoming migration event",
							slog.String(LogVirtualMachineId, msg.Msg.Dom.Name),
							slog.String("event", eventKey),
						)
					}

					// Default successful path for lifecycle events
					slog.DebugContext(
						ctx, "Virtual machine lifecycle event detected",
						slog.String(LogVirtualMachineId, msg.Msg.Dom.Name),
						slog.String("event", eventKey),
					)
					enqueueVMByName(ctx, connections, msg.Msg.Dom.Name, "", false, "", jobQueue)

				}
			}
		}
	}
}

// StartNATSListener subscribes to NATS Disk QoS update events and orchestrates enforcements.
func StartNATSListener(ctx context.Context, connections *controllers.Connections, jobQueue chan<- EnforcementJob) {
	slog.InfoContext(ctx, "NATS event listener started")

	if connections.NatsInstance == nil || connections.NatsInstance.JetStreamConnection == nil {
		slog.ErrorContext(ctx, "NATS connection not initialized. Listener aborting.")
		return
	}

	if err := createIotuneStreams(ctx, connections.NatsInstance); err != nil {
		slog.ErrorContext(ctx, "Failed to create streams for IOtune subscriptions", slog.String(LogMessage, err.Error()))
		return
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = uuid.NewString()
	}

	consumer, err := connections.NatsInstance.InitConsumer(
		DiskStreamName,
		[]string{DiskSubjectSubscription},
		true,
		fmt.Sprintf("va-iotune-%s", hostname),
	)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to subscribe to disk qos update", slog.String(LogMessage, err.Error()))
		return
	}

	var wg sync.WaitGroup

	qosConsume, err := consumer.Consume(func(msg jetstream.Msg) {
		wg.Add(1)
		defer wg.Done()

		var incomingMessage NATSDiskQoSUpdateRequest
		if err := json.Unmarshal(msg.Data(), &incomingMessage); err != nil {
			slog.ErrorContext(ctx, "Failed to parse NATS message", slog.String(LogMessage, err.Error()))
			msg.Term()
			return
		}

		slog.InfoContext(ctx, "Received QoS update request via NATS", slog.String("volume_uuid", incomingMessage.VolumeUUID), slog.String("request_id", incomingMessage.RequestID))

		// Enqueue job specifying the exact disk serial to enforce.
		enqueueVMByName(ctx, connections, incomingMessage.InstanceUUID, incomingMessage.VolumeUUID, true, incomingMessage.RequestID, jobQueue)

		msg.Ack()
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to start NATS consumer consumption", slog.String(LogMessage, err.Error()))
		return
	}

	<-ctx.Done()
	qosConsume.Stop()
	wg.Wait()
	slog.InfoContext(ctx, "NATS event listener stopped")
}

// createIotuneStreams creates the required NATS streams if they don't exist.
func createIotuneStreams(ctx context.Context, n *messaging.NATSInstance) error {
	streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := n.JetStreamConnection.CreateOrUpdateStream(streamCtx, jetstream.StreamConfig{
		Name:      DiskStreamName,
		Retention: jetstream.InterestPolicy,
		Replicas:  3,
		Subjects: []string{
			DiskSubjectSubscription,
			DiskSubjectNotification,
		},
		Compression: jetstream.S2Compression,
	})
	if err != nil {
		return err
	}

	_, err = n.JetStreamConnection.CreateOrUpdateKeyValue(streamCtx, jetstream.KeyValueConfig{
		Bucket:       KeyValueEnforcementIntentBucket,
		Description:  "Accepted disk qos intents last enforced by the agent",
		MaxValueSize: 1024 * 1024 * 1024,
		History:      10,
		Storage:      jetstream.FileStorage,
		Replicas:     3,
		Compression:  true,
	})
	return err
}

// enqueueVMByName fetches the VM struct using libvirt client and enqueues an EnforcementJob.
func enqueueVMByName(
	ctx context.Context,
	connections *controllers.Connections,
	vmName string, targetDiskSerial string, triggeredByNATS bool, natsRequestID string, jobQueue chan<- EnforcementJob) {
	vm, err := connections.LibvirtInstance.GetLocalVM(ctx, vmName)
	if err != nil {
		if triggeredByNATS {
			slog.InfoContext(ctx, "VM does not exist here, nothing to do",
				slog.String("vm_name", vmName),
				slog.String(LogMessage, err.Error()),
			)
		} else {
			slog.ErrorContext(ctx, "Failed to fetch VM details for queueing",
				slog.String("vm_name", vmName),
				slog.String(LogMessage, err.Error()),
			)
		}
		return
	}

	reqCtx := context.WithValue(ctx, LogGlobalReqID, uuid.NewString())

	select {
	case <-ctx.Done():
	case jobQueue <- EnforcementJob{
		VM:               vm,
		TargetDiskSerial: targetDiskSerial,
		TriggeredByNATS:  triggeredByNATS,
		NATSRequestID:    natsRequestID,
		Context:          reqCtx,
	}:
		slog.DebugContext(reqCtx, "Enqueued VM for enforcement from event", slog.String("vm_name", vmName))
	}
}
