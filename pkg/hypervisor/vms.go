package hypervisor

import (
	"context"
	"encoding/xml"
	"log/slog"
	"reflect"
	"strings"

	"github.com/digitalocean/go-libvirt"
	"golang.org/x/sync/errgroup"
)

// GetLocalVM queries the libvirt daemon for a specific domain by its hypervisor name.
// It retrieves the domain's raw XML description and unmarshals it into a strongly-typed
// VirtualMachine struct, providing immediate access to the VM's hardware inventory.
func (l *LibvirtClient) GetLocalVM(ctx context.Context, name string) (VirtualMachine, error) {
	libvm, err := l.Connection.DomainLookupByName(name)
	if err != nil {
		return VirtualMachine{}, err
	}
	return l.GetLocalVMByDomain(ctx, libvm)
}

// GetLocalVMByDomain bypasses the lookup phase for instances where the libvirt.Domain
// reference is already known (e.g., when iterating over a pre-fetched domain list).
func (l *LibvirtClient) GetLocalVMByDomain(ctx context.Context, libvm libvirt.Domain) (VirtualMachine, error) {
	libVmXml, err := l.Connection.DomainGetXMLDesc(libvm, 0)
	if err != nil {
		return VirtualMachine{}, err
	}

	virtualMachine := VirtualMachine{DomainRef: libvm}

	if err := xml.Unmarshal([]byte(libVmXml), &virtualMachine); err != nil {
		return VirtualMachine{}, err
	}

	return virtualMachine, nil
}

// ListLocalVMs retrieves a list of all domains running on the local compute node.
// The status parameter filters the results and can be "active", "inactive", or "all".
func (l *LibvirtClient) ListLocalVMs(ctx context.Context, status string) ([]VirtualMachine, error) {

	var flags libvirt.ConnectListAllDomainsFlags
	status = strings.ToLower(status)

	switch status {
	case "active":
		flags = libvirt.ConnectListDomainsActive
	case "inactive":
		flags = libvirt.ConnectListDomainsInactive
	default:
		flags = libvirt.ConnectListDomainsActive | libvirt.ConnectListDomainsInactive
	}

	vms, _, err := l.Connection.ConnectListAllDomains(1, flags)
	if err != nil {
		return nil, err
	}

	virtualMachines := make([]VirtualMachine, len(vms))

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(5)

	for i, vm := range vms {
		i, vm := i, vm
		eg.Go(func() error {
			if err := egCtx.Err(); err != nil {
				return err
			}

			virtualMachine, err := l.GetLocalVMByDomain(egCtx, vm)
			if err != nil {
				slog.ErrorContext(
					egCtx, "Libvirt Domain Describe Failure",
					slog.String("virtual_machine_name", vm.Name),
					slog.String("error", err.Error()),
				)
				return nil // Continue gathering other VMs
			}
			virtualMachines[i] = virtualMachine
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Filter out any VMs that failed to describe
	var result []VirtualMachine
	for _, vm := range virtualMachines {
		if vm.UUID != "" {
			result = append(result, vm)
		}
	}

	return result, nil
}

// SetVMDiskIOPS applies the specified IOTune QoS policy to a running virtual machine's disk.
// It uses reflection to dynamically map the generic IOTune struct fields to libvirt's TypedParam array,
// applying the limits dynamically to both the live guest and its persistent configuration.
func (l *LibvirtClient) SetVMDiskIOPS(ctx context.Context, virtualMachine VirtualMachine, ioPolicy IOTune, disk string) error {

	params := []libvirt.TypedParam{}

	val := reflect.ValueOf(ioPolicy)
	typ := reflect.TypeOf(ioPolicy)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Extract the XML tag to determine the exact libvirt parameter name.
		rawTag := fieldType.Tag.Get("xml")
		if rawTag == "" {
			continue
		}

		// Strip options like ",omitempty" to get the raw parameter key.
		paramName := strings.Split(rawTag, ",")[0]

		// Map the field values into the typed parameter structure required by libvirt RPC.
		switch field.Kind() {

		case reflect.Uint64:
			value := field.Uint()
			// Only apply if > 0. Libvirt treats 0 as "unlimited" usually,
			// but we generally want to avoid sending 0s unless explicitly intended.
			if value > 0 {
				params = append(params, libvirt.TypedParam{
					Field: paramName,
					Value: libvirt.TypedParamValue{
						D: uint32(libvirt.TypedParamUllong),
						I: value,
					},
				})
			}
		case reflect.String:
			value := field.String()
			if value != "" {
				params = append(params, libvirt.TypedParam{
					Field: paramName,
					Value: libvirt.TypedParamValue{
						D: uint32(libvirt.TypedParamString),
						I: value,
					},
				})
			}
		}
	}

	if len(params) == 0 {
		return nil
	}

	flags := libvirt.DomainAffectLive | libvirt.DomainAffectConfig
	if err := l.Connection.DomainSetBlockIOTune(virtualMachine.DomainRef, disk, params, uint32(flags)); err != nil {
		return err
	}

	return nil
}
