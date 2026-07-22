package iotune

import (
	"context"
	"log/slog"

	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/config"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/controllers"
	"github.com/aravindh-murugesan/openstack-virt-agent/pkg/hypervisor"
)

// EnforcementJob represents a single work item for the IOTune controller worker pool.
type EnforcementJob struct {
	VM               hypervisor.VirtualMachine
	TargetDiskSerial string // If provided, limits enforcement to this specific disk.
	TriggeredByNATS  bool
	NATSRequestID    string // Included if TriggeredByNATS is true
	Context          context.Context
}

// StartWorkerPool initializes the channel and spawns the specified number of worker goroutines.
func StartWorkerPool(
	ctx context.Context,
	numWorkers int,
	connections *controllers.Connections,
	cfg *config.IOPSConfig,
) chan<- EnforcementJob {

	jobQueue := make(chan EnforcementJob, 100) // Buffer size can be tweaked

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			slog.DebugContext(ctx, "Starting IOtune worker", slog.Int("worker_id", workerID))
			for {
				select {
				case <-ctx.Done():
					slog.DebugContext(ctx, "Stopping IOtune worker", slog.Int("worker_id", workerID))
					return
				case job, ok := <-jobQueue:
					if !ok {
						return // Channel closed
					}

					// Delegate to the core enforcer logic
					EnforceVM(job.Context, job.VM, job.TargetDiskSerial, connections, cfg, job.TriggeredByNATS, job.NATSRequestID)
				}
			}
		}(i)
	}

	return jobQueue
}
