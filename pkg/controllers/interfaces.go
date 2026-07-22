package controllers

import (
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/hypervisor"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/messaging"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/openstack"
)

// Controller is the interface that all virt-agent controllers must implement.
type Controller interface {
	Name() string
	IsEnabled() bool
	StartDaemon() error
	StopDaemon() error
}

// Connections holds initialized shared clients required by the controllers.
type Connections struct {
	NatsInstance      *messaging.NATSInstance
	OpenstackInstance *openstack.OpenstackClient
	LibvirtInstance   *hypervisor.LibvirtClient
}
