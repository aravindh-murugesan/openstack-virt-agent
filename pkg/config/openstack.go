package config

type OpenStackConfig struct {
	CloudsFile string `mapstructure:"clouds_file"`
	CloudName  string `mapstructure:"cloud_name"`
}
