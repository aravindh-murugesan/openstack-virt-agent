package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// Validate runs cross-field semantic validation on the configuration.
func (c *Config) Validate() error {
	if err := c.Controllers.IOPS.Validate(); err != nil {
		return fmt.Errorf("iops controller config error: %w", err)
	}
	if err := c.Nats.Validate(); err != nil {
		return fmt.Errorf("nats config error: %w", err)
	}
	return nil
}

// Validate checks the NATS configuration.
func (n *NatsConfig) Validate() error {
	if n.Enabled && n.Server == "" {
		return fmt.Errorf("server address is required when nats is enabled")
	}
	return nil
}

// Validate checks the IOPS controller configuration.
func (i *IOPSConfig) Validate() error {
	if !i.Enabled {
		return nil
	}

	// Validate public keys
	for name, b64key := range i.AuthorizedPubKeys {
		keyBytes, err := base64.StdEncoding.DecodeString(b64key)
		if err != nil {
			return fmt.Errorf("invalid base64 encoding for authorized key '%s'", name)
		}

		pub, err := x509.ParsePKIXPublicKey(keyBytes)
		if err != nil {
			// Fallback to checking if it's a raw 32-byte key
			if len(keyBytes) != ed25519.PublicKeySize {
				return fmt.Errorf("authorized key '%s' is neither a valid SPKI key (%v) nor a raw %d-byte ed25519 key (length: %d)", name, err, ed25519.PublicKeySize, len(keyBytes))
			}
		} else {
			if _, ok := pub.(ed25519.PublicKey); !ok {
				return fmt.Errorf("authorized key '%s' is a valid SPKI key but not an ed25519 public key", name)
			}
		}
	}

	// Validate volume type policies (Max >= Base)
	for vt, policy := range i.VolumeTypePolicy {
		if err := validatePolicyCaps(policy.BasePolicy, policy.MaxPolicy); err != nil {
			return fmt.Errorf("volume_type_policy '%s' validation failed: %w", vt, err)
		}
	}

	return nil
}

func validatePolicyCaps(base, max Policy) error {
	if max.TotalIopsSec > 0 && max.TotalIopsSec < base.TotalIopsSec {
		return fmt.Errorf("max_policy.total_iops_sec (%d) cannot be less than base_policy.total_iops_sec (%d)", max.TotalIopsSec, base.TotalIopsSec)
	}
	if max.ReadIopsSec > 0 && max.ReadIopsSec < base.ReadIopsSec {
		return fmt.Errorf("max_policy.read_iops_sec (%d) cannot be less than base_policy.read_iops_sec (%d)", max.ReadIopsSec, base.ReadIopsSec)
	}
	if max.WriteIopsSec > 0 && max.WriteIopsSec < base.WriteIopsSec {
		return fmt.Errorf("max_policy.write_iops_sec (%d) cannot be less than base_policy.write_iops_sec (%d)", max.WriteIopsSec, base.WriteIopsSec)
	}
	if max.TotalBytesSec > 0 && max.TotalBytesSec < base.TotalBytesSec {
		return fmt.Errorf("max_policy.total_bytes_sec (%d) cannot be less than base_policy.total_bytes_sec (%d)", max.TotalBytesSec, base.TotalBytesSec)
	}
	if max.ReadBytesSec > 0 && max.ReadBytesSec < base.ReadBytesSec {
		return fmt.Errorf("max_policy.read_bytes_sec (%d) cannot be less than base_policy.read_bytes_sec (%d)", max.ReadBytesSec, base.ReadBytesSec)
	}
	if max.WriteBytesSec > 0 && max.WriteBytesSec < base.WriteBytesSec {
		return fmt.Errorf("max_policy.write_bytes_sec (%d) cannot be less than base_policy.write_bytes_sec (%d)", max.WriteBytesSec, base.WriteBytesSec)
	}
	return nil
}
