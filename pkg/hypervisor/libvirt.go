package hypervisor

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
)

// LibvirtClient provides an interface to interact with the local hypervisor daemon.
// It wraps the underlying go-libvirt connection to perform hardware-level operations
// such as querying domains and modifying runtime disk I/O policies.
type LibvirtClient struct {
	Connection         *libvirt.Libvirt
	URI                string
	PkiPath            string
	InsecureSkipVerify bool
	Hostname           string
}

// NewLibvirtClient establishes a connection to the local or remote libvirt daemon using the provided URI.
// It parses the URI and negotiates the RPC connection via a local socket or TLS dialer.
func NewLibvirtClient(uri string, pkiPath string, insecureSkipVerify bool) (*LibvirtClient, error) {
	validatedurl, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	var conn *libvirt.Libvirt
	if validatedurl.Scheme == "qemu+tls" {
		opts := []dialers.TLSOption{
			dialers.UsePKIPath(pkiPath),
		}
		if insecureSkipVerify {
			opts = append(opts, dialers.WithInsecureNoVerify())
		}

		conn = libvirt.NewWithDialer(
			dialers.NewTLS(validatedurl.Host, opts...),
		)

		err = conn.Connect()
		if err != nil {
			return nil, err
		}

	} else {
		conn, err = libvirt.ConnectToURI(validatedurl)
		if err != nil {
			return nil, err
		}
	}

	hostname, err := conn.ConnectGetHostname()
	if err != nil {
		return nil, err
	}

	return &LibvirtClient{
		Connection:         conn,
		URI:                uri,
		PkiPath:            pkiPath,
		InsecureSkipVerify: insecureSkipVerify,
		Hostname:           hostname,
	}, nil
}

// CheckHealth verifies if the connection to libvirtd is still alive.
func (l *LibvirtClient) CheckHealth() error {
	if !l.Connection.IsConnected() {
		return fmt.Errorf("libvirt connection (%s) is no longer alive", l.URI)
	}

	_, err := l.Connection.ConnectGetVersion()
	if err != nil {
		return fmt.Errorf("libvirt connection (%s) is no longer alive: %v", l.URI, err)
	}

	return nil
}

// Reconnect attempts to re-establish the connection to the libvirt daemon.
func (l *LibvirtClient) Reconnect() error {
	// Attempt to close the old connection if it exists (ignore errors)
	if l.Connection != nil {
		_ = l.Connection.Disconnect()
	}

	nlc, err := NewLibvirtClient(l.URI, l.PkiPath, l.InsecureSkipVerify)
	if err != nil {
		return err
	}

	l.Connection = nlc.Connection
	l.Hostname = nlc.Hostname
	return nil
}

// EnsureConnection checks the health of the libvirt connection and attempts to reconnect if it is dead.
// It returns an error if the connection cannot be established, allowing the caller to back off and retry later.
func (l *LibvirtClient) EnsureConnection() error {
	if err := l.CheckHealth(); err == nil {
		return nil // connection is healthy
	}

	slog.Warn("Libvirt connection lost, attempting to reconnect", slog.String("uri", l.URI))
	if err := l.Reconnect(); err != nil {
		return fmt.Errorf("reconnect failed: %w", err)
	}

	slog.Info("Successfully reconnected to libvirt daemon", slog.String("uri", l.URI))
	return nil
}

// SubscribeEvents registers for a specific type of libvirt event and returns a channel
// where these events will be published. It wraps the underlying go-libvirt subscription.
func (l *LibvirtClient) SubscribeEvents(ctx context.Context, eventID libvirt.DomainEventID, dom libvirt.OptDomain) (<-chan interface{}, error) {
	return l.Connection.SubscribeEvents(ctx, eventID, dom)
}
