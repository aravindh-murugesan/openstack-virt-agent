package iotune

import (
	"context"
	"log/slog"
	"time"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/config"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers"
	"github.com/google/uuid"
)

// StartAuditLoop begins the periodic audit reconciliation process, polling Libvirt
// for active VMs and queuing them up for enforcement.
func StartAuditLoop(ctx context.Context, connections *controllers.Connections, cfg *config.IOPSConfig, jobQueue chan<- EnforcementJob) {
	interval := time.Duration(cfg.AuditIntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 30 * time.Minute // default fallback
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "Audit loop started", slog.String(LogController, "iotune"), slog.String("interval", interval.String()))

	// Run an immediate initial pass before waiting on the ticker
	runAuditPass(ctx, connections, jobQueue)

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Audit loop stopped")
			return
		case <-ticker.C:
			slog.InfoContext(ctx, "Audit interval reached", slog.String("interval", interval.String()))
			runAuditPass(ctx, connections, jobQueue)
		}
	}
}

func runAuditPass(parentCtx context.Context, connections *controllers.Connections, jobQueue chan<- EnforcementJob) {
	// Job context carries values but NO 30s timeout
	jobCtx := context.WithValue(parentCtx, LogGlobalReqID, uuid.NewString())
	jobCtx = context.WithValue(jobCtx, LogControllerComponentHandler, "audit_reconciliation")

	// Local context bound to 30s strictly for gathering the VMs and queuing them
	auditCtx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	slog.DebugContext(jobCtx, "Starting Audit Reconciliation pass")

	if err := connections.LibvirtInstance.EnsureConnection(); err != nil {
		slog.ErrorContext(jobCtx, "Libvirt connection is down, skipping audit pass", slog.String(LogMessage, err.Error()))
		return
	}

	slog.DebugContext(jobCtx, "Attempting to list active virtual machines during audit pass")
	vms, err := connections.LibvirtInstance.ListLocalVMs(auditCtx, "active")
	if err != nil {
		slog.ErrorContext(jobCtx, "Failed to list active virtual machines during audit pass", slog.String(LogMessage, err.Error()))
		return
	}
	slog.InfoContext(jobCtx, "Fetched active virtual machines during audit pass")

	for _, vm := range vms {
		select {
		case <-auditCtx.Done():
			slog.WarnContext(jobCtx, "Audit pass context cancelled before all VMs were queued")
			return
		case jobQueue <- EnforcementJob{
			VM:      vm,
			Context: jobCtx, // Workers now use this unbounded context
		}:
			slog.DebugContext(jobCtx, "Virtual machine queued for IO Audit Reconciliation",
				slog.String(LogVirtualMachineName, vm.Name),
				slog.String(LogVirtualMachineId, vm.UUID),
			)
		}
	}

	slog.InfoContext(jobCtx, "Completed Audit Reconciliation pass", slog.Int("vms_queued", len(vms)))
}
