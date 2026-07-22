package iotune

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/messaging"
	"github.com/nats-io/nats.go/jetstream"
)

// AcceptedIntent represents the validated intent state persisted to NATS KV.
type AcceptedIntent struct {
	Value     string `json:"value"`
	Signature string `json:"signature"`
	Requestor string `json:"requestor"`
}

// GetLastAcceptedIntent retrieves the last successfully enforced intent from the NATS KV store.
func GetLastAcceptedIntent(ctx context.Context, natsInstance *messaging.NATSInstance, volumeID string) (*AcceptedIntent, error) {
	if natsInstance == nil || natsInstance.JetStreamConnection == nil {
		return nil, errors.New("NATS JetStream connection is not initialized")
	}

	kv, err := natsInstance.JetStreamConnection.KeyValue(ctx, KeyValueEnforcementIntentBucket)
	if err != nil {
		return nil, err
	}

	entry, err := kv.Get(ctx, volumeID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, nil // Normal case where no override history exists
		}
		return nil, err
	}

	var intent AcceptedIntent
	if err := json.Unmarshal(entry.Value(), &intent); err != nil {
		return nil, err
	}

	return &intent, nil
}

// StoreLastAcceptedIntent stores the validated intent in the NATS KV store.
func StoreLastAcceptedIntent(ctx context.Context, natsInstance *messaging.NATSInstance, volumeID string, intent *AcceptedIntent) error {
	if natsInstance == nil || natsInstance.JetStreamConnection == nil {
		return errors.New("NATS JetStream connection is not initialized")
	}

	if intent == nil || intent.Value == "" {
		return nil
	}

	storeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	kv, err := natsInstance.JetStreamConnection.KeyValue(storeCtx, KeyValueEnforcementIntentBucket)
	if err != nil {
		return err
	}

	data, err := json.Marshal(intent)
	if err != nil {
		return err
	}

	_, err = kv.Put(storeCtx, volumeID, data)
	if err != nil {
		slog.ErrorContext(storeCtx, "Failed to persist last accepted intent in NATS KV",
			slog.String(LogCinderDiskID, volumeID),
			slog.String(LogMessage, err.Error()),
		)
		return err
	}

	return nil
}
