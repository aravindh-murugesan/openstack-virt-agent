package config

type Config struct {
	Environment string            `mapstructure:"environment"`
	LogLevel    string            `mapstructure:"log_level"`
	Controllers ControllersConfig `mapstructure:"controllers"`
	Nats        NatsConfig        `mapstructure:"nats"`
	OpenStack   OpenStackConfig   `mapstructure:"openstack"`
}
