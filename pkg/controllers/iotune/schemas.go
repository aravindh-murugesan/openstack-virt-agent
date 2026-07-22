package iotune

import (
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/hypervisor"
)

// NATSDiskQoSUpdateRequest represents the event payload we expect to receive when 
// there's a request to enforce QoS via NATS.
type NATSDiskQoSUpdateRequest struct {
	RequestID    string `json:"request_id"`
	InstanceUUID string `json:"instance_uuid"`
	VolumeUUID   string `json:"volume_uuid"`
}

// NATSDiskQoSEnforcementNotification represents the event payload we publish 
// after an enforcement attempt (successful or failed).
type NATSDiskQoSEnforcementNotification struct {
	ID             string             `json:"id"`
	RequestID      string             `json:"request_id,omitempty"`
	InstanceUUID   string             `json:"instance_uuid"`
	InstanceName   string             `json:"instance_name"`
	VolumeUUID     string             `json:"volume_uuid"`
	OccurredAt     time.Time          `json:"occurred_at"`
	Status         string             `json:"status"`
	Message        string             `json:"message,omitempty"`
	EnforcedPolicy hypervisor.IOTune  `json:"enforced_policy,omitempty"`
}
