package config

type IOPSConfig struct {
	Enabled              bool                        `mapstructure:"enabled"`
	ValidateIntent       bool                        `mapstructure:"validate_intent"`
	GlobalBasePolicy     Policy                      `mapstructure:"global_base_policy"`
	VolumeTypePolicy     map[string]VolumeTypePolicy `mapstructure:"volume_type_policy"`
	AuthorizedPubKeys    map[string]string           `mapstructure:"authorized_pub_keys"`
	AuditIntervalMinutes int                         `mapstructure:"audit_interval_minutes"`
}

type VolumeTypePolicy struct {
	BasePolicy Policy `mapstructure:"base_policy"`
	MaxPolicy  Policy `mapstructure:"max_policy"`
}

type Policy struct {
	TotalIopsSec     uint64 `mapstructure:"total_iops_sec"`
	ReadIopsSec      uint64 `mapstructure:"read_iops_sec"`
	WriteIopsSec     uint64 `mapstructure:"write_iops_sec"`
	TotalBytesSec    uint64 `mapstructure:"total_bytes_sec"`
	ReadBytesSec     uint64 `mapstructure:"read_bytes_sec"`
	WriteBytesSec    uint64 `mapstructure:"write_bytes_sec"`
	TotalIopsSecMax  uint64 `mapstructure:"total_iops_sec_max"`
	ReadIopsSecMax   uint64 `mapstructure:"read_iops_sec_max"`
	WriteIopsSecMax  uint64 `mapstructure:"write_iops_sec_max"`
	TotalBytesSecMax uint64 `mapstructure:"total_bytes_sec_max"`
	ReadBytesSecMax  uint64 `mapstructure:"read_bytes_sec_max"`
	WriteBytesSecMax uint64 `mapstructure:"write_bytes_sec_max"`
	SizeIopsSec      uint64 `mapstructure:"size_iops_sec"`
	GroupName        string `mapstructure:"group_name"`
}
