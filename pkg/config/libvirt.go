package config

// LibvirtConfig allows both TLS and Local QEMU connections.
// When URI is set to anything other than "qemu:///system", it is assumed that the connection is over TLS.
type LibvirtConfig struct {
	URI                string `mapstructure:"uri"`
	PkiPath            string `mapstructure:"pki_path"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}
