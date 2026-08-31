package config

type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

type NatsConfig struct {
	Enabled  bool      `mapstructure:"enabled"`
	Server   string    `mapstructure:"server"`
	Username string    `mapstructure:"username"`
	Password string    `mapstructure:"password"`
	TLS      TLSConfig `mapstructure:"tls"`
}
