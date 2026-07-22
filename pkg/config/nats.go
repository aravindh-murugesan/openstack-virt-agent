package config

type NatsConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Server   string `mapstructure:"server"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}
