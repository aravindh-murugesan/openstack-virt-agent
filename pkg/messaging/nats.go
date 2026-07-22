// Package messaging provides a structured wrapper around NATS and JetStream connections
// to simplify messaging patterns and client identification.
package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSInstance holds the state for a NATS connection and its associated JetStream context.
// It serves as a centralized container for managing connectivity and metadata
// for a specific NATS client.
type NATSInstance struct {
	// Connection is the underlying primary NATS client connection.
	Connection *nats.Conn
	// JetStreamConnection provides the context for interacting with NATS JetStream capabilities.
	JetStreamConnection jetstream.JetStream
	// URL is the server address (e.g., "nats://localhost:4222") the instance is connected to.
	URL string
}

func NewNATSInstance(c config.NatsConfig) (*NATSInstance, error) {

	n := NATSInstance{
		URL: c.Server,
	}

	opts := []nats.Option{
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			slog.Warn("NATS connection disconnected", slog.Any("error", err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS connection restored")
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			slog.Warn("NATS connection completely closed")
		}),
	}

	if c.Server == "" {
		return nil, fmt.Errorf("nats server URL is not specified")
	}

	if (c.Username == "" && c.Password != "") || (c.Username != "" && c.Password == "") {
		return nil, fmt.Errorf("nats username and password must be set together")
	} else if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}

	ns, err := nats.Connect(n.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %w", n.URL, err)
	}

	n.Connection = ns

	js, err := jetstream.New(ns)
	if err != nil {
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	n.JetStreamConnection = js
	return &n, nil
}

// InitConsumer sets up a JetStream consumer on a specified stream.
// If durable is true, the consumer will persist using the ClientIdentifier as the durable name,
// allowing it to resume after a disconnect.
func (n *NATSInstance) InitConsumer(streamName string, subjects []string, durable bool, clientIdentifier string) (jetstream.Consumer, error) {

	if n.Connection == nil || n.JetStreamConnection == nil {
		return nil, fmt.Errorf("either NATS or Jetstream connections are not initialized properly to start consumers")
	}

	pctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := n.JetStreamConnection.Stream(pctx, streamName)
	if err != nil {
		return nil, err
	}

	consumerConfig := jetstream.ConsumerConfig{
		Name:           clientIdentifier,
		DeliverPolicy:  jetstream.DeliverAllPolicy,
		AckPolicy:      jetstream.AckExplicitPolicy,
		FilterSubjects: subjects,
	}

	if durable {
		consumerConfig.Durable = clientIdentifier
	}

	consumer, err := stream.CreateOrUpdateConsumer(pctx, consumerConfig)
	if err != nil {
		return nil, err
	}

	return consumer, err
}

// InitBucket sets up a JetStream KeyValue store bucket.
func (n *NATSInstance) InitBucket(bucketName string, description string, replicas int) (jetstream.KeyValue, error) {
	if n.Connection == nil || n.JetStreamConnection == nil {
		return nil, fmt.Errorf("either NATS or Jetstream connections are not initialized properly")
	}

	if replicas > 3 {
		replicas = 3
	}

	pctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	kv, err := n.JetStreamConnection.CreateOrUpdateKeyValue(pctx, jetstream.KeyValueConfig{
		Bucket:       bucketName,
		Description:  description,
		MaxValueSize: 1024 * 1024, // 1MB
		History:      10,
		Storage:      jetstream.FileStorage,
		Replicas:     replicas,
		Compression:  true,
	})

	return kv, err
}

func (n *NATSInstance) CheckHealth() error {
	if !n.Connection.IsConnected() || !n.JetStreamConnection.Conn().IsConnected() {
		return fmt.Errorf("NATS connectivity has been interrupted")
	}
	return nil
}
