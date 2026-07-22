package hypervisor

import (
	"fmt"

	"github.com/digitalocean/go-libvirt"
)

// ParseLifecycle converts raw Libvirt lifecycle event IDs and detail IDs
// into human-readable strings.
//
// It returns two strings: a constant-like "Event Key" (e.g., "VM_STARTED")
// and a descriptive "Detail Message" that includes the specific reason
// and the original detail code.
func ParseLifecycle(eventID int32, detailID int32) (string, string) {
	// Cast to Libvirt types for readable switch cases
	event := libvirt.DomainEventType(eventID)

	switch event {
	case libvirt.DomainEventDefined:
		// 0: Defined
		switch libvirt.DomainEventDefinedDetailType(detailID) {
		case libvirt.DomainEventDefinedAdded:
			return "VM_DEFINED", fmt.Sprintf("Created/Added (XML) [Code: %d]", detailID)
		case libvirt.DomainEventDefinedUpdated:
			return "VM_DEFINED", fmt.Sprintf("Updated Configuration [Code: %d]", detailID)
		case libvirt.DomainEventDefinedRenamed:
			return "VM_DEFINED", fmt.Sprintf("Renamed [Code: %d]", detailID)
		case libvirt.DomainEventDefinedFromSnapshot:
			return "VM_DEFINED", fmt.Sprintf("Restored Config from Snapshot [Code: %d]", detailID)
		default:
			return "VM_DEFINED", fmt.Sprintf("Unknown Definition Change [Code: %d]", detailID)
		}

	case libvirt.DomainEventUndefined:
		// 1: Undefined
		switch libvirt.DomainEventUndefinedDetailType(detailID) {
		case libvirt.DomainEventUndefinedRemoved:
			return "VM_REMOVED", fmt.Sprintf("Configuration Removed [Code: %d]", detailID)
		case libvirt.DomainEventUndefinedRenamed:
			return "VM_REMOVED", fmt.Sprintf("Renamed (Old Config Gone) [Code: %d]", detailID)
		default:
			return "VM_REMOVED", fmt.Sprintf("Undefined [Code: %d]", detailID)
		}

	case libvirt.DomainEventStarted:
		// 2: Started
		switch libvirt.DomainEventStartedDetailType(detailID) {
		case libvirt.DomainEventStartedBooted:
			return "VM_STARTED", fmt.Sprintf("Booted [Code: %d]", detailID)
		case libvirt.DomainEventStartedMigrated:
			return "VM_STARTED", fmt.Sprintf("Incoming Migration [Code: %d]", detailID)
		case libvirt.DomainEventStartedRestored:
			return "VM_STARTED", fmt.Sprintf("Restored from File [Code: %d]", detailID)
		case libvirt.DomainEventStartedFromSnapshot:
			return "VM_STARTED", fmt.Sprintf("Restored from Snapshot [Code: %d]", detailID)
		case libvirt.DomainEventStartedWakeup:
			return "VM_STARTED", fmt.Sprintf("Woke Up [Code: %d]", detailID)
		default:
			return "VM_STARTED", fmt.Sprintf("Started (Unknown Reason) [Code: %d]", detailID)
		}

	case libvirt.DomainEventSuspended:
		// 3: Suspended
		switch libvirt.DomainEventSuspendedDetailType(detailID) {
		case libvirt.DomainEventSuspendedPaused:
			return "VM_SUSPENDED", fmt.Sprintf("Paused by User [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedMigrated:
			return "VM_SUSPENDED", fmt.Sprintf("Paused for Migration [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedIoerror:
			return "VM_SUSPENDED", fmt.Sprintf("Paused (I/O Error) [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedWatchdog:
			return "VM_SUSPENDED", fmt.Sprintf("Paused (Watchdog Trigger) [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedRestored:
			return "VM_SUSPENDED", fmt.Sprintf("Restored in Paused State [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedFromSnapshot:
			return "VM_SUSPENDED", fmt.Sprintf("Snapshot Restored (Paused) [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedAPIError:
			return "VM_SUSPENDED", fmt.Sprintf("Paused (API Error) [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedPostcopy:
			return "VM_SUSPENDED", fmt.Sprintf("Paused (Post-Copy Migration) [Code: %d]", detailID)
		case libvirt.DomainEventSuspendedPostcopyFailed:
			return "VM_SUSPENDED", fmt.Sprintf("Paused (Post-Copy Failed) [Code: %d]", detailID)
		default:
			return "VM_SUSPENDED", fmt.Sprintf("Suspended (Unknown Reason) [Code: %d]", detailID)
		}

	case libvirt.DomainEventResumed:
		// 4: Resumed
		switch libvirt.DomainEventResumedDetailType(detailID) {
		case libvirt.DomainEventResumedUnpaused:
			return "VM_RESUMED", fmt.Sprintf("Unpaused [Code: %d]", detailID)
		case libvirt.DomainEventResumedMigrated:
			return "VM_RESUMED", fmt.Sprintf("Resumed (Migration Finished) [Code: %d]", detailID)
		case libvirt.DomainEventResumedFromSnapshot:
			return "VM_RESUMED", fmt.Sprintf("Resumed from Snapshot [Code: %d]", detailID)
		default:
			return "VM_RESUMED", fmt.Sprintf("Resumed (Unknown Reason) [Code: %d]", detailID)
		}

	case libvirt.DomainEventStopped:
		// 5: Stopped
		switch libvirt.DomainEventStoppedDetailType(detailID) {
		case libvirt.DomainEventStoppedShutdown:
			return "VM_STOPPED", fmt.Sprintf("Shutdown (Graceful) [Code: %d]", detailID)
		case libvirt.DomainEventStoppedDestroyed:
			return "VM_STOPPED", fmt.Sprintf("Destroyed (Force Off) [Code: %d]", detailID)
		case libvirt.DomainEventStoppedCrashed:
			return "VM_STOPPED", fmt.Sprintf("Crashed [Code: %d]", detailID)
		case libvirt.DomainEventStoppedMigrated:
			return "VM_STOPPED", fmt.Sprintf("Migrated Away [Code: %d]", detailID)
		case libvirt.DomainEventStoppedSaved:
			return "VM_STOPPED", fmt.Sprintf("Saved to File [Code: %d]", detailID)
		case libvirt.DomainEventStoppedFailed:
			return "VM_STOPPED", fmt.Sprintf("Host Failure [Code: %d]", detailID)
		case libvirt.DomainEventStoppedFromSnapshot:
			return "VM_STOPPED", fmt.Sprintf("Snapshot Revert [Code: %d]", detailID)
		default:
			return "VM_STOPPED", fmt.Sprintf("Stopped (Unknown Reason) [Code: %d]", detailID)
		}

	case libvirt.DomainEventShutdown:
		// 6: Shutdown (The process is finishing)
		switch libvirt.DomainEventShutdownDetailType(detailID) {
		case libvirt.DomainEventShutdownFinished:
			return "VM_SHUTDOWN", fmt.Sprintf("Shutdown Finished [Code: %d]", detailID)
		default:
			return "VM_SHUTDOWN", fmt.Sprintf("Shutdown (Unknown) [Code: %d]", detailID)
		}

	case libvirt.DomainEventPmsuspended:
		// 7: PMSuspended (Guest S3/S4)
		return "VM_PMSUSPENDED", fmt.Sprintf("Guest PM Suspend [Code: %d]", detailID)

	case libvirt.DomainEventCrashed:
		// 8: Crashed (QEMU Process died)
		switch libvirt.DomainEventCrashedDetailType(detailID) {
		case libvirt.DomainEventCrashedPanicked:
			return "VM_CRASHED", fmt.Sprintf("Kernel Panic [Code: %d]", detailID)
		default:
			return "VM_CRASHED", fmt.Sprintf("Process Crashed [Code: %d]", detailID)
		}

	default:
		return "UNKNOWN_EVENT", fmt.Sprintf("Event: %d | Detail: %d", eventID, detailID)
	}
}
