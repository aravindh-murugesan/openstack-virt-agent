package hypervisor

import (
	"encoding/xml"

	"github.com/digitalocean/go-libvirt"
)

// VirtualMachine represents the top-level <domain> configuration in Libvirt XML.
// It maps the critical metadata required to identify and manage an OpenStack instance.
type VirtualMachine struct {
	// DomainRef is the RPC handle to the domain.
	// NOTE: This is NOT populated by xml.Unmarshal. It must be set manually
	// via l.DomainLookupByName() or l.Domains().
	DomainRef libvirt.Domain

	XMLName xml.Name `xml:"domain"`
	// Type is the virtualization type, typically "kvm" or "qemu".
	Type string `xml:"type,attr"`
	// ID is the running hypervisor ID (changes on reboot).
	ID string `xml:"id,attr"`
	// Name is the internal libvirt name (e.g., "instance-0000001").
	Name string `xml:"name"`
	// UUID is the immutable identifier for the VM (matches OpenStack Instance UUID).
	UUID string `xml:"uuid"`
	// Memory configuration for the VM.
	Memory Memory `xml:"memory"`
	// VCPUs is the count of virtual CPUs allocated.
	VCPUs int `xml:"vcpu"`
	// Devices contains the hardware inventory (disks, interfaces, etc.).
	Devices Devices `xml:"devices"`
	// Host name from openstack metadata
	OpenstackName string `xml:"metadata>instance>name"`
}

// Memory defines the RAM allocation.
type Memory struct {
	// Unit is usually "KiB".
	Unit  string `xml:"unit,attr"`
	Value int    `xml:",chardata"`
}

// Disk represents a block device attached to the VM.
type Disk struct {

	// Type is the backing type: "file" (qcow2), "block" (LVM/Cinder), or "network" (Ceph).
	Type string `xml:"type,attr"`
	// Device is the emulation type, usually "disk" or "cdrom".
	Device string `xml:"device,attr"`
	// Driver contains backend driver details (qemu, raw/qcow2, cache mode).
	Driver DiskDriver `xml:"driver"`
	// Source describes the host-side path or network URI.
	Source DiskSource `xml:"source"`
	// Target describes how the guest sees the device (e.g., vda).
	Target DiskTarget `xml:"target"`
	// IOTune contains the current Quality of Service (QoS) limits.
	IOTune IOTune `xml:"iotune"`
	// Serial is the unique ID. For OpenStack, this matches the Cinder Volume UUID.
	Serial string `xml:"serial"`
	// Alias is the internal libvirt device alias (e.g., "ua-ec99...").
	Alias string `xml:"alias"`
}

// DiskDriver describes the hypervisor driver settings.
type DiskDriver struct {
	// Name is usually "qemu".
	Name string `xml:"name,attr"`
	// Type is the format: "raw" or "qcow2".
	Type string `xml:"type,attr"`
	// Cache mode: "none", "writeback", "writethrough", etc.
	Cache string `xml:"cache,attr"`
	// IO mode: "native" or "threads".
	IO string `xml:"io,attr"`
}

// DiskSource maps the host-side storage location.
type DiskSource struct {
	// Dev is used for block devices (Type="block"). E.g., "/dev/dm-5".
	Dev string `xml:"dev,attr"`
	// File is used for file-backed disks (Type="file").
	File string `xml:"file,attr"`
	// Index is used in some newer libvirt versions for array referencing.
	Index string `xml:"index,attr"`
}

// DiskTarget maps the guest-side device name.
type DiskTarget struct {
	// Dev is the device name the Guest OS sees (e.g., "vda", "sda").
	// This is the identifier required for DomainSetBlockIOTune commands.
	Dev string `xml:"dev,attr"`
	// Bus is the controller type: "virtio", "scsi", "ide", "usb".
	Bus string `xml:"bus,attr"`
}

// IOTune defines storage I/O throttling limits (QoS).
// This struct serves a dual purpose:
// 1. Parsing current limits from XML (using `xml` tags).
// 2. Generating parameters for RPC calls (using `libvirt` tags).
type IOTune struct {
	// Basic throughput and IOPS limits
	TotalBytesSec uint64 `xml:"total_bytes_sec,omitempty" yaml:"total_bytes_sec,omitempty"`
	ReadBytesSec  uint64 `xml:"read_bytes_sec,omitempty" yaml:"read_bytes_sec,omitempty"`
	WriteBytesSec uint64 `xml:"write_bytes_sec,omitempty" yaml:"write_bytes_sec,omitempty"`
	TotalIopsSec  uint64 `xml:"total_iops_sec,omitempty" yaml:"total_iops_sec,omitempty"`
	ReadIopsSec   uint64 `xml:"read_iops_sec,omitempty" yaml:"read_iops_sec,omitempty"`
	WriteIopsSec  uint64 `xml:"write_iops_sec,omitempty" yaml:"write_iops_sec,omitempty"`

	// Burst limits (Max)
	TotalBytesSecMax uint64 `xml:"total_bytes_sec_max,omitempty" yaml:"total_bytes_sec_max,omitempty"`
	ReadBytesSecMax  uint64 `xml:"read_bytes_sec_max,omitempty" yaml:"read_bytes_sec_max,omitempty"`
	WriteBytesSecMax uint64 `xml:"write_bytes_sec_max,omitempty" yaml:"write_bytes_sec_max,omitempty"`
	TotalIopsSecMax  uint64 `xml:"total_iops_sec_max,omitempty" yaml:"total_iops_sec_max,omitempty"`
	ReadIopsSecMax   uint64 `xml:"read_iops_sec_max,omitempty" yaml:"read_iops_sec_max,omitempty"`
	WriteIopsSecMax  uint64 `xml:"write_iops_sec_max,omitempty" yaml:"write_iops_sec_max,omitempty"`

	// SizeIopsSec penalizes large I/O by counting them as multiple IOPS.
	// E.g., if 4k, a 1MB I/O counts as 256 IOPS, thus clamping abuses.
	SizeIopsSec uint64 `xml:"size_iops_sec,omitempty" yaml:"size_iops_sec,omitempty"`

	// GroupName is used to share limits across multiple disks.
	GroupName string `xml:"group_name,omitempty" yaml:"group_name,omitempty"`
}

// Interface defines the network configuration.
type Interface struct {
	// Type is usually "ethernet" (generic) or "bridge".
	Type   string          `xml:"type,attr"`
	MAC    MAC             `xml:"mac"`
	Target InterfaceTarget `xml:"target"`
	Model  InterfaceModel  `xml:"model"`
	Driver InterfaceDriver `xml:"driver"`
	MTU    MTU             `xml:"mtu"`
}

// MAC address of the interface.
type MAC struct {
	Address string `xml:"address,attr"`
}

// InterfaceTarget defines the host-side TAP device.
type InterfaceTarget struct {
	// Dev is the TAP device name (e.g., "tap143deaa2-07").
	Dev string `xml:"dev,attr"`
}

// InterfaceModel defines the emulated hardware card.
type InterfaceModel struct {
	// Type is usually "virtio" for performance.
	Type string `xml:"type,attr"`
}

// InterfaceDriver defines the backend network driver.
type InterfaceDriver struct {
	// Name is usually "vhost" (kernel acceleration).
	Name string `xml:"name,attr"`
	// Queues is the number of multi-queues enabled (should match vCPUs for best performance).
	Queues int `xml:"queues,attr"`
}

// MTU defines the Maximum Transmission Unit.
type MTU struct {
	// Size is typically 1500 (standard) or 9000 (jumbo).
	// OpenStack generic setups often use 1442 or 1450 to account for VXLAN/Geneve overhead.
	Size int `xml:"size,attr"`
}

// Devices is a container for all hardware components.
type Devices struct {
	Disks      []Disk      `xml:"disk"`
	Interfaces []Interface `xml:"interface"`
}
