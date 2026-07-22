package openstack

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
)

func (o *OpenstackClient) GetVolume(ctx context.Context, volumeID string) (volumes.Volume, string, error) {
	opCtx, opCancel := context.WithTimeout(ctx, time.Duration(o.OperationTimeoutSeconds)*time.Second)
	defer opCancel()

	slog.DebugContext(opCtx, "Attempting to fetch volume information", slog.String("cinder_disk_id", volumeID))

	resp := volumes.Get(opCtx, o.BlockStorageClient, volumeID)
	reqID := resp.Header.Get("X-Openstack-Request-Id")
	if reqID == "" {
		reqID = uuid.NewString()
	}

	v, err := resp.Extract()
	if err != nil {
		slog.ErrorContext(opCtx, "Failed to get volume information",
			slog.String("cinder_disk_id", volumeID),
			slog.String("error", err.Error()),
		)
		return volumes.Volume{}, reqID, err
	}

	slog.DebugContext(opCtx, "Successfully fetched volume information",
		slog.String("cinder_disk_id", volumeID),
		slog.String("cinder_disk_state", v.Status),
	)

	return *v, reqID, nil
}

func (o *OpenstackClient) AddVolumeMetadata(
	ctx context.Context,
	volumeID string,
	metadata map[string]string,
) (volumes.Volume, string, error) {
	opCtx, opCancel := context.WithTimeout(ctx, time.Duration(o.OperationTimeoutSeconds)*time.Second)
	defer opCancel()

	slog.DebugContext(opCtx, "Attempting to add volume metadata", slog.String("cinder_disk_id", volumeID))

	v, volGetRequestID, err := o.GetVolume(opCtx, volumeID)
	if err != nil {
		return volumes.Volume{}, volGetRequestID, err
	}

	existingMetadata := v.Metadata
	if existingMetadata == nil {
		existingMetadata = make(map[string]string)
	}

	newMetadata := maps.Clone(existingMetadata)
	maps.Copy(newMetadata, metadata)

	updateOpts := volumes.UpdateOpts{
		Metadata: newMetadata,
	}

	updateResult := volumes.Update(opCtx, o.BlockStorageClient, volumeID, updateOpts)
	reqID := updateResult.Header.Get("X-Openstack-Request-Id")

	updatedVol, uerr := updateResult.Extract()
	if uerr != nil {
		slog.ErrorContext(opCtx, "Failed to update volume metadata",
			slog.String("cinder_disk_id", volumeID),
			slog.String("error", uerr.Error()),
		)
		return volumes.Volume{}, reqID, uerr
	}

	slog.InfoContext(opCtx, "Successfully updated volume metadata",
		slog.String("cinder_disk_id", volumeID),
		slog.String("cinder_disk_state", updatedVol.Status),
		slog.Any("cinder_disk_metadata", newMetadata),
	)

	return *updatedVol, reqID, nil
}
