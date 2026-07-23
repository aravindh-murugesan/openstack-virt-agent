package iotune

const (
	IOPOLICYOVERRIDEKEY          = "x-virt-enforcer-io-policy-override"
	IOPOLICYOVERRIDESIGNATUREKEY = "x-virt-enforcer-io-policy-signature"
	IOPOLICYINTENTREQUESTOR      = "x-virt-enforcer-io-policy-intent-requestor"

	LogRequestID                  = "request_id"
	LogGlobalReqID                = "global_request_id"
	LogController                 = "controller"
	LogControllerComponentHandler = "controller_handler"
	LogVirtualMachineId           = "virtual_machine_id"
	LogVirtualMachineName         = "virtual_machine_name"
	LogCinderDiskID               = "cinder_disk_id"
	LogCinderVolumeType           = "cinder_volume_type"
	LogCinderDiskState            = "cinder_disk_state"
	LogMessage                    = "message"
	LogCloudRequestID             = "cloud_request_id"

	KeyValueEnforcementIntentBucket = "cinder-disk-qos-enforcement-accepted-intent"

	DiskStreamName                = "cinder-disk-qos"
	DiskSubjectSubscription       = "cinder.disk.qos.subs"
	DiskSubjectNotification       = "cinder.disk.qos.enforcement.notifications.*"
	DiskSubjectNotificationPrefix = "cinder.disk.qos.enforcement.notifications."
)
